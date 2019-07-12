/**
 * Description:
 * Author: Yihen.Liu
 * Create: 2019-04-26
 */
package kcp

import (
	"bufio"
	"fmt"
	"github.com/saveio/porter/common"
	"github.com/saveio/porter/internal/protobuf"
	"github.com/saveio/themis/common/log"
	"github.com/xtaci/kcp-go"
	"math/rand"
	"net"
	"sync"
	"time"
	"encoding/hex"
)

const (
	MONITOR_TIME_INTERVAL = 3
	PEER_MONITOR_TIMEOUT  = 10 * time.Second
	MESSAGE_CHANNEL_LEN   = 1024
	LISTEN_CHANNEL_LEN    = 1024
)

type port struct {
	start uint32
	end   uint32
	used  uint32
}

type peer struct {
	addr       string
	conn       net.Conn
	listener   *kcp.Listener
	loginTime  time.Time
	updateTime time.Time
	stop       chan struct{}
	state      *ConnState
}
type msgNotify struct {
	message *protobuf.Message
	state   *ConnState
}

type peerListen struct {
	connectionID string
	state        *ConnState
}

type KcpProxyServer struct {
	mainListener   *kcp.Listener
	proxies        *sync.Map
	ports          port
	msgBuffer      chan msgNotify
	listenerBuffer chan peerListen
}

type ConnState struct {
	conn         net.Conn
	writer       *bufio.Writer
	messageNonce uint64
	writerMutex  *sync.Mutex
	stop         chan struct{}
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func Init() *KcpProxyServer {
	return &KcpProxyServer{
		proxies:        new(sync.Map),
		msgBuffer:      make(chan msgNotify, MESSAGE_CHANNEL_LEN),
		listenerBuffer: make(chan peerListen, LISTEN_CHANNEL_LEN),
	}
}

func newConnState(conn net.Conn) *ConnState {
	return &ConnState{
		conn:         conn,
		writer:       bufio.NewWriterSize(conn, defaultRecvBufferSize),
		messageNonce: 0,
		writerMutex:  new(sync.Mutex),
		stop:         make(chan struct{}),
	}
}

// Listen listens for incoming UDP connections on a specified port.
func listen(ip string, port uint16) (*kcp.Listener, error) {
	resolved := fmt.Sprintf("%s:%d", ip, port)
	listener, err := kcp.ListenWithOptions(resolved, nil, 0, 0)

	if err != nil {
		return nil, err
	}

	return listener, nil
}

func (p *KcpProxyServer) kcpServerListenAndAccept(ip string, port uint16) {
	var err error
	p.mainListener, err = listen(ip, port)
	if err != nil {
		log.Errorf("kcp server listen start ERROR:", err.Error(), "listen(IP/PORT):%s,%d",ip,port)
	} else {
		log.Info("Kcp Proxy Listen IP:", p.mainListener.Addr().String())
	}
	go p.serverAccept()
	go p.handleControlMessage()
	go p.startListenScheduler()
}

func (p *KcpProxyServer) serverAccept() error {
	for {
		conn, err := p.mainListener.Accept()
		if err != nil {
			log.Error("kcp listener accept error:", err.Error())
		}else {
			log.Info("Kcp main listener accept a client connection, client-addr:",conn.RemoteAddr().String())
		}
		go func() {
			connState := newConnState(conn)
			for {
				message, err := receiveMessage(connState)
				log.Infof("accept kcp connection(remote addr:%s) receive a message, message.opcode:%d,message.Sender.Addr:%s, message.sign:%s",conn.RemoteAddr().String(), message.Opcode, message.Sender.Address, hex.EncodeToString(message.Signature))
				if nil == message || err != nil {
					break
				}
				p.msgBuffer <- msgNotify{message: message, state: connState}
			}
		}()
	}
	return nil
}

func (p *KcpProxyServer) monitorPeerStatus() {
	interval := time.Tick(MONITOR_TIME_INTERVAL * time.Second)
	for {
		select {
		case <-interval:
			p.proxies.Range(func(key, value interface{}) bool {
				if time.Now().After(value.(peer).updateTime.Add(PEER_MONITOR_TIMEOUT)) {
					p.releasePeerResource(key.(string))
					log.Info("monitor timeout, proxy service release, peerID:", key.(string))
				}
				return true
			})
		}
	}
}

func (p *KcpProxyServer) StartKCPServer(port uint16) {
	go p.monitorPeerStatus()
	go p.kcpServerListenAndAccept(common.GetLocalIP(), port)
	<- make(chan struct{})
}
