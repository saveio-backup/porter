/**
 * Description:
 * Author: Yihen.Liu
 * Create: 2019-04-26 
*/
package kcp

import (
	"net"
	"sync"
	"github.com/saveio/themis/common/log"
	"math/rand"
	"time"
	"github.com/saveio/porter/types/opcode"
	"github.com/saveio/porter/common"
	"github.com/xtaci/kcp-go"
	"fmt"
	"bufio"
)

const (
	MONITOR_TIME_INTERVAL  	= 3
	PEER_MONITOR_TIMEOUT	= 10 * time.Second
)
type port struct {
	start 	uint32
	end 	uint32
	used 	uint32
}

type peer struct {
	addr		string
	conn 		net.Conn
	listener	*kcp.Listener
	loginTime 	time.Time
	updateTime 	time.Time
	stop 		chan struct{}
}

type KcpProxyServer struct {
	listener 	*kcp.Listener
	proxies 	*sync.Map
	ports 		port
}

type ConnState struct {
	conn         net.Conn
	writer       *bufio.Writer
	messageNonce uint64
	writerMutex  *sync.Mutex
}

func init()  {
	rand.Seed(time.Now().UnixNano())
}

func Init() *KcpProxyServer {
	return &KcpProxyServer{
		proxies:new(sync.Map),
	}
}

func newConnState(conn net.Conn) *ConnState {
	return &ConnState{
		conn:conn,
		writer:bufio.NewWriterSize(conn, defaultRecvBufferSize),
		messageNonce:0,
		writerMutex:new(sync.Mutex),
	}
}
// Listen listens for incoming UDP connections on a specified port.
func listen(ip string, port uint16)( *kcp.Listener,error) {
	resolved:=fmt.Sprintf("%s:%d",ip,port)
	listener, err := kcp.ListenWithOptions(resolved , nil, 0, 0)

	if err != nil {
		return nil, err
	}

	return listener,nil
}

func(p *KcpProxyServer) kcpServerListenAndAccept(ip string, port uint16) {
	var err error
	p.listener,err = listen(ip, port)
	if err!=nil{
		log.Errorf("kcp server listen start ERROR:", err.Error())
	}else{
		log.Info("Kcp Proxy Listen IP:", p.listener.Addr().String())
	}
	p.serverAccept()
}

func (p *KcpProxyServer)proxyListenAndAccept(peerID string, conn net.Conn) string {
	port:= uint16(rand.Intn(10000)+55635)
	listener, err:=listen(common.GetLocalIP(), port)
	if err!=nil{
		log.Error("proxy listen server start ERROR:", err.Error())
		return ""
	}

	peerInfo := peer{addr:fmt.Sprintf("kcp://%s:%d", common.GetLocalIP(),port),
					conn:conn,
					listener:listener,
					loginTime:time.Now(),
					updateTime:time.Now(),
					stop:make(chan struct{}),
				}
	p.proxies.Store(peerID, peerInfo)

	go p.proxyAccept(peerInfo)
	return fmt.Sprintf("%s:%d", common.GetLocalIP(), port)
}

func (p *KcpProxyServer) proxyAccept(peerInfo peer) error {
	for{
		conn ,err := peerInfo.listener.Accept()
		if err!=nil{
			log.Error("peer proxy accept err:", err.Error())
			continue
		}
		go func() {
			for{
				buffer, _:= receiveKCPRawMessage(conn)
				transferKCPRawMessage(buffer, peerInfo.conn)
			}
		}()
	}
	return nil
}

func(p *KcpProxyServer) kcpConnectionAccept() error {
	return nil
}

func (p *KcpProxyServer) serverAccept() error {
	for {
		conn, err := p.listener.Accept()
		if err!=nil{
			log.Error("kcp listener accept error:", err.Error())
		}
		connState := newConnState(conn)
		go func() {
			for{
				message, err := receiveMessage(conn)
				if nil==message || err!=nil {
					break
				}
				switch message.Opcode {
				case uint32(opcode.ProxyRequestCode):
					p.handleProxyRequestMessage(message, connState)
				case uint32(opcode.KeepaliveCode):
					p.handleProxyKeepaliveMessage(message, connState)
				case uint32(opcode.DisconnectCode):
					p.handleDisconnectMessage(message)
				}
			}
		}()
	}
	return nil
}

func (p *KcpProxyServer) monitorPeerStatus()  {
	interval := time.Tick( MONITOR_TIME_INTERVAL * time.Second)
	for {
		select {
		case <-interval:
			p.proxies.Range(func(key, value interface{}) bool {
				if time.Now().After(value.(peer).updateTime.Add(PEER_MONITOR_TIMEOUT)){
					p.releasePeerResource(key.(string))
					log.Info("client has disconnect from proxy server, peerID:", key.(string))
				}
				return true
			})
		}
	}
}

func(p *KcpProxyServer) StartKCPServer(port uint16) {
	//go p.monitorPeerStatus()
	p.kcpServerListenAndAccept(common.GetLocalIP(),port)
}
