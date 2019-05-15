/**
 * Description:
 * Author: Yihen.Liu
 * Create: 2019-04-26 
*/
package udp

import (
	"net"
	"strconv"
	"sync"
	"github.com/saveio/themis/common/log"
	"math/rand"
	"time"
	"github.com/saveio/porter/types/opcode"
	"github.com/saveio/porter/common"
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
	conn 		*net.UDPConn
	loginTime 	time.Time
	updateTime 	time.Time
	stop 		chan struct{}
}

type ProxyServer struct {
	listener 	*net.UDPConn
	proxies 	*sync.Map
	ports 		port
}

func init()  {
	rand.Seed(time.Now().UnixNano())
}

func Init() *ProxyServer {
	return &ProxyServer{
		proxies:new(sync.Map),
	}
}

// Listen listens for incoming UDP connections on a specified port.
func listen(ip string, port uint16)( *net.UDPConn,error) {
	resolved, err := net.ResolveUDPAddr("udp", ip + ":"+strconv.Itoa(int(port)))
	if err != nil {
		return nil, err
	}
	listener, err := net.ListenUDP("udp", resolved)
	if err != nil {
		return nil, err
	}

	return listener,nil
}

func(p *ProxyServer) serverListenAndAccept(ip string, port uint16)  {
	var err error
	p.listener,err = listen(ip, port)
	if err!=nil{
		log.Errorf("server listen start ERROR:", err.Error())
	}else{
		log.Info("Proxy Listen IP:",ip,", Port:",port)
	}
	p.serverAccept()
}

func (p *ProxyServer)publicIpForSpecProxy(listener*net.UDPConn, port uint16) string {
	if common.Parameters.PublicIP == ""{
		return listener.LocalAddr().String()
	}
	return fmt.Sprintf("%s:%d", common.Parameters.PublicIP,port)
}

func (p *ProxyServer)proxyListenAndAccept(peerID string, remoteAddr string) string {
	randomPort := common.RandomPort("udp")
	listener, err:=listen(common.GetLocalIP(), randomPort)
	if err!=nil{
		log.Error("proxy listen server start ERROR:", err.Error())
		return ""
	}
	log.Info("proxy-listen:", listener.LocalAddr().String())

	peerInfo := peer{addr:remoteAddr,
					conn:listener,
					loginTime:time.Now(),
					updateTime:time.Now(),
					stop:make(chan struct{}),
				}
	p.proxies.Store(peerID, peerInfo)

	go p.proxyAccept(peerInfo)
	return p.publicIpForSpecProxy(listener, randomPort)
}

func (p *ProxyServer) proxyAccept(peerInfo peer) error {
	for {
		select {
		case <-peerInfo.stop:
			return nil
		default:
			if message, err := receiveUDPRawMessage(peerInfo.conn); err == nil{
				transferUDPRawMessage(message, p.listener, peerInfo.addr)
			}else{
				return err
			}
		}
	}
	return nil
}

func (p *ProxyServer) serverAccept() error {
	for {
		message, remoteAddr := receiveUDPProxyMessage(p.listener)
		if nil==message {
			continue
		}
		switch message.Opcode {
		case uint32(opcode.ProxyRequestCode):
			p.handleProxyRequestMessage(message, remoteAddr)
		case uint32(opcode.KeepaliveCode):
			p.handleProxyKeepaliveMessage(message)
		case uint32(opcode.DisconnectCode):
			p.handleDisconnectMessage(message)
		}
	}
	return nil
}

func (p *ProxyServer) monitorPeerStatus()  {
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

func(p *ProxyServer) StartUDPServer() {
	go p.monitorPeerStatus()
	go p.serverListenAndAccept(common.GetLocalIP(),6008)
}
