/**
 * Description:
 * Author: Yihen.Liu
 * Create: 2019-04-26
 */
package tcp

import (
	"bufio"
	"fmt"
	"github.com/saveio/porter/common"
	"github.com/saveio/porter/internal/protobuf"
	"github.com/saveio/themis/common/log"
	"sync"
	"time"
	mRand "math/rand"
	"net"
	"github.com/saveio/porter/types/opcode"
)

const (
	MONITOR_TIME_INTERVAL = 3
	PEER_MONITOR_TIMEOUT  = 10 * time.Second
	MESSAGE_CHANNEL_LEN   = 65535
	LISTEN_CHANNEL_LEN    = 65535
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
	stop 			chan struct{}
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
		stop:			make(chan struct{}),
	}
}

func newConnState(listenr net.Conn, addr string) *ConnState {
	return &ConnState{
		conn:         listenr,
		writer:       bufio.NewWriterSize(listenr, defaultRecvBufferSize),
		messageNonce: 0,
		writerMutex:  new(sync.Mutex),
		stop:         make(chan struct{}),
		remoteAddr:	  addr,
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
			log.Error("tcp listener accept error:", err.Error(), "listen addr:", p.mainListener.Addr().String())
			continue
		}
		go func(conn net.Conn) {
			connState := newConnState(conn,conn.RemoteAddr().String())
			for {
				message, err := receiveMessage(connState)
				if nil == message || err != nil {
					log.Error("tcp receive message goroutine err:", err.Error(), "listen remote addr:",conn.RemoteAddr().String())
					p.releasePeerResource(connState.connectionID)
					break
				}
				if message.Opcode == uint32(opcode.ProxyRequestCode) || message.Opcode == uint32(opcode.KeepaliveCode) {
					p.msgBuffer <- msgNotify{message: message, state: connState}
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
					log.Info("client has disconnect from proxy server, peerID:", key.(string))
				}
				return true
			})
		}
	}
}

func (p *TCPProxyServer) StartTCPServer(port uint16) {
	go p.monitorPeerStatus()
	go p.tcpServerListenAndAccept(common.GetLocalIP(), port)
	<- make(chan struct{})
}
