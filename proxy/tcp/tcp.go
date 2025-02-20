/**
 * Description:
 * Author: Yihen.Liu
 * Create: 2019-04-26
 */
package tcp

import (
	"bufio"
	"fmt"
	mRand "math/rand"
	"net"
	"sync"
	"time"

	"os"
	"os/signal"
	"syscall"

	"github.com/saveio/porter/common"
	"github.com/saveio/porter/internal/protobuf"
	"github.com/saveio/porter/types/opcode"
	"github.com/saveio/themis/common/log"
)

const (
	MONITOR_TIME_INTERVAL   = 60
	PEER_MONITOR_TIMEOUT    = 180 * time.Second
	DEFAULT_PORT_CACHE_TIME = 7200
	MESSAGE_CHANNEL_LEN     = 65535
	LISTEN_CHANNEL_LEN      = 65535
)

type port struct {
	start uint32
	end   uint32
	used  uint32
}

type peer struct {
	addr       string
	conn       net.Conn
	listener   net.Listener
	loginTime  time.Time
	updateTime time.Time
	stop       chan struct{}
	state      *ConnState
	release    *sync.Once
}
type msgNotify struct {
	message *protobuf.Message
	state   *ConnState
}

type peerListen struct {
	connectionID string
	state        *ConnState
}

type TCPProxyServer struct {
	mainListener   net.Listener
	proxies        *sync.Map
	ports          port
	msgBuffer      chan msgNotify
	listenerBuffer chan peerListen
	stop           chan struct{}
	Metric         common.PorterMetric
}

type ConnState struct {
	conn         net.Conn
	writer       *bufio.Writer
	messageNonce uint64
	writerMutex  *sync.Mutex
	stop         chan struct{}
	connectionID string
	remoteAddr   string
}

func init() {
	mRand.Seed(time.Now().UnixNano())
}

func Init() *TCPProxyServer {
	return &TCPProxyServer{
		proxies:        new(sync.Map),
		msgBuffer:      make(chan msgNotify, MESSAGE_CHANNEL_LEN),
		listenerBuffer: make(chan peerListen, LISTEN_CHANNEL_LEN),
		stop:           make(chan struct{}),
		//Metric:         common.InitMetrics(),
	}
}

func newConnState(listenr net.Conn, addr string) *ConnState {
	return &ConnState{
		conn:         listenr,
		writer:       bufio.NewWriterSize(listenr, common.Parameters.WriteBufferSize),
		messageNonce: 0,
		writerMutex:  new(sync.Mutex),
		stop:         make(chan struct{}),
		remoteAddr:   addr,
	}
}

// Listen listens for incoming UDP connections on a specified port.
func listen(ip string, port uint16) (net.Listener, error) {
	resolved := fmt.Sprintf("%s:%d", ip, port)
	listener, err := net.Listen("tcp", resolved)

	if err != nil {
		return nil, err
	}

	return listener, nil
}

func (p *TCPProxyServer) tcpServerListenAndAccept(ip string, port uint16) {
	var err error
	p.mainListener, err = listen(ip, port)
	if err != nil {
		log.Errorf("tcp server listen start ERROR:", err.Error())
		return
	} else {
		log.Info("TCP Proxy Listen IP:", p.mainListener.Addr().String())
	}
	go p.serverAccept()
	go p.handleControlMessage()
	go p.startListenScheduler()
}

func (p *TCPProxyServer) serverAccept() error {
	for {
		conn, err := p.mainListener.Accept()
		if err != nil {
			log.Warn("tcp listener accept error:", err.Error(), "listen addr:", p.mainListener.Addr().String())
			continue
		}
		//p.Metric.MainConnCounter.Inc(1)
		go func(conn net.Conn) {
			log.Info("start a new goroutine for new Inbound connection in main proxy server accept, remote addr:", conn.RemoteAddr().String())
			connState := newConnState(conn, conn.RemoteAddr().String())
			firstInboundMsg := true
			for {
				message, err := receiveMessage(connState)
				if nil == message || err != nil {
					log.Warn("tcp receive message goroutine err:", err.Error(), "listen remote addr:", conn.RemoteAddr().String())
					if firstInboundMsg == true || connState.connectionID == "" {
						log.Error("first inbound message is error, connection will be closed immediately.")
						//p.Metric.MainConnCounter.Dec(1)
						conn.Close()
						break
					}
					p.releasePeerResource(connState.connectionID)
					break
				}
				log.Info("receive a new message which need to be controll message type in main Accept, message.opcode:",
					message.Opcode, "netID:", message.NetID, "sender address:", message.Sender.Address)
				if message.Opcode == uint32(opcode.ProxyRequestCode) || message.Opcode == uint32(opcode.KeepaliveCode) || message.Opcode == uint32(opcode.MetricRequestCode) {
					p.msgBuffer <- msgNotify{message: message, state: connState}
					firstInboundMsg = false
				}
			}
		}(conn)
	}
	close(p.stop)
	return nil
}

func (p *TCPProxyServer) monitorPeerStatus() {
	interval := time.Tick(MONITOR_TIME_INTERVAL * time.Second)
	for {
		select {
		case <-interval:
			p.proxies.Range(func(key, value interface{}) bool {
				if time.Now().After(value.(peer).updateTime.Add(PEER_MONITOR_TIMEOUT)) {
					p.releasePeerResource(key.(string))
					log.Info("client has disconnect from proxy server as for monitor timeout, proxy-addr:", value.(peer).addr,
						",monitor timeout second:", PEER_MONITOR_TIMEOUT, ",lastst update time:", value.(peer).updateTime)
				}
				return true
			})

			timeout := common.Parameters.PortTimeout
			if common.Parameters.PortTimeout <= 0 {
				timeout = DEFAULT_PORT_CACHE_TIME
			}
			common.PortSet.Cache.Range(func(key, value interface{}) bool {
				if time.Now().After(value.(*common.UsingPort).Timestamp.Add(timeout * time.Second)) {
					delKey := fmt.Sprintf("%s-%s", value.(*common.UsingPort).Protocol, value.(*common.UsingPort).ConnectionID)
					common.PortSet.Cache.Delete(delKey)
					log.Info("proxy port timeout in memory cache:", delKey, " delete now. latest timestamp:", value.(*common.UsingPort).Timestamp)
				}
				return true
			})
		}
	}
}

func (p *TCPProxyServer) waitExit() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	select {
	case sig := <-sigs:
		log.Infof("TCPProxyServer received exit signal:%v,", sig.String(), ",begin to release all resource.")
		p.proxies.Range(func(key, value interface{}) bool {
			p.releasePeerResource(key.(string))
			return true
		})
		p.mainListener.Close()
		common.PortSet.PorterDB.Close()
		os.Exit(0)
	}
}

func (p *TCPProxyServer) StartTCPServer(port uint16) {
	go p.monitorPeerStatus()
	go p.tcpServerListenAndAccept(common.GetLocalIP(), port)
	go p.waitExit()
	<-make(chan struct{})
}
