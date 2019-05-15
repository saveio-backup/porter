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
)

const (
	MAX_PACKAGE_SIZE        = 1024 * 64
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
	loginTime 	time.Time
	updateTime 	time.Time
	stop 		chan struct{}
}

type KcpProxyServer struct {
	listener 	*kcp.Listener
	proxies 	*sync.Map
	ports 		port
}

func init()  {
	rand.Seed(time.Now().UnixNano())
}

func Init() *KcpProxyServer {
	return &KcpProxyServer{
		proxies:new(sync.Map),
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

func (p *KcpProxyServer)proxyListenAndAccept(peerID string, listener net.Conn) string {
	log.Info("proxy-listen:", listener.LocalAddr().String())

	peerInfo := peer{addr:listener.RemoteAddr().String(),
					conn:listener,
					loginTime:time.Now(),
					updateTime:time.Now(),
					stop:make(chan struct{}),
				}
	p.proxies.Store(peerID, peerInfo)

	go p.proxyAccept(peerInfo)
	return listener.LocalAddr().String()
}

func (p *KcpProxyServer) proxyAccept(peerInfo peer) error {
	for {
		select {
		case <-peerInfo.stop:
			return nil
		default:
			if message, err := receiveKCPRawMessage(peerInfo.conn); err == nil{
				transferKCPRawMessage(message, peerInfo.conn)
			}else{
				return err
			}
		}
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
		}else{
			fmt.Println("server accept, peer addr:", conn.RemoteAddr().String())
		}
		message, err := receiveMessage(conn)
		if nil==message || err!=nil {
			continue
		}
		switch message.Opcode {
		case uint32(opcode.ProxyRequestCode):
			p.handleProxyRequestMessage(message, conn)
		case uint32(opcode.KeepaliveCode):
			p.handleProxyKeepaliveMessage(message)
		case uint32(opcode.DisconnectCode):
			p.handleDisconnectMessage(message)
		}
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

func(p *KcpProxyServer) StartKCPServer() {
	//go p.monitorPeerStatus()
	p.kcpServerListenAndAccept(common.GetLocalIP(),6008)
}
