/**
 * Description:
 * Author: Yihen.Liu
 * Create: 2019-04-26 
*/
package proxy

import (
	"net"
	"strconv"
	"sync"
	"fmt"
	"net/url"
	"github.com/saveio/themis/common/log"
	"math/rand"
	"time"
	"github.com/saveio/porter/internal/protobuf"
	"encoding/hex"
	"github.com/saveio/porter/types/opcode"
	"github.com/saveio/porter/common"
)

const (
	MAX_PACKAGE_SIZE        = 1024 * 64
)
type port struct {
	start 	uint32
	end 	uint32
	used 	uint32
}

type peer struct {
	addr	string
	conn 	*net.UDPConn
}

type ProxyServer struct {
	listener 	*net.UDPConn
	proxies 	*sync.Map
	ports 		port
}

// AddressInfo represents a network URL.
type addressInfo struct {
	Protocol string
	Host     string
	Port     uint16
}

func(addr addressInfo) toString() string {
	return fmt.Sprintf("%s:%d", addr.Host, addr.Port)
}

func init()  {
	rand.Seed(time.Now().UnixNano())
}

func Init() *ProxyServer {
	return &ProxyServer{
		proxies:new(sync.Map),
	}
}

// ParseAddress derives a network scheme, host and port of a destinations
// information. Errors should the provided destination address be malformed.
//protocol://ip:port
func ParseAddress(address string) (*addressInfo, error) {
	urlInfo, err := url.Parse(address)
	if err != nil {
		return nil, err
	}

	host, rawPort, err := net.SplitHostPort(urlInfo.Host)
	if err != nil {
		return nil, err
	}

	port, err := strconv.ParseUint(rawPort, 10, 16)
	if err != nil {
		return nil, err
	}

	return &addressInfo{
		Protocol: urlInfo.Scheme,
		Host:     host,
		Port:     uint16(port),
	}, nil
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

func (p *ProxyServer)proxyListenAndAccept(peerID string, remoteAddr string) string {
	listener, err:=listen(common.GetLocalIP(), uint16(rand.Intn(10000)+55635))
	if err!=nil{
		log.Error("proxy listen server start ERROR:", err.Error())
		return ""
	}
	log.Info("proxy-listen:", listener.LocalAddr().String())
	p.proxies.Store(peerID, peer{addr:remoteAddr, conn:listener})
	go p.proxyAccept(listener, remoteAddr)
	return listener.LocalAddr().String()
}

func (p *ProxyServer) proxyAccept(conn *net.UDPConn, remoteAddr string) error {
	for {
		message := receiveUDPRawMessage(conn)
		if nil==message{
			continue
		}
		transferUDPRawMessage(message, p.listener, remoteAddr)
	}
	return nil
}

func (p *ProxyServer) serverAccept() error {
	for {
		message, remoteAddr := receiveUDPProxyMessage(p.listener)
		if nil==message {
			continue
		}
		if message.Opcode != uint32(opcode.ProxyRequestCode) {
			continue
		}

		//if the client is working in public-net environment, return ip address directly
		if message.Sender.Address == remoteAddr{
			addrInfo, err:=ParseAddress(remoteAddr)
			if err!=nil{
				log.Error("parse remoteAddr err:", err.Error())
				continue
			}
			sendUDPMessage(&protobuf.ProxyResponse{ProxyAddress:addrInfo.toString()}, p.listener, remoteAddr)
			continue
		}

		var proxyIP string
		peerID := hex.EncodeToString(message.Sender.Id)

		peerInfo, ok:=p.proxies.Load(peerID)
		if !ok{
			proxyIP = p.proxyListenAndAccept(peerID, remoteAddr) //DialAddress应该是对方的公网IP, receiveUDPMessage的时候返回
			sendUDPMessage(&protobuf.ProxyResponse{ProxyAddress:proxyIP}, p.listener, remoteAddr)
		} else if peerInfo.(peer).addr != message.DialAddress {
			p.proxies.Delete(peerID)
			proxyIP = p.proxyListenAndAccept(peerID, message.DialAddress)
			sendUDPMessage(&protobuf.ProxyResponse{ProxyAddress:proxyIP}, p.listener, remoteAddr)
		}
	}
	return nil
}

func(p *ProxyServer) StartServer() {
	p.serverListenAndAccept(common.GetLocalIP(),6008)
}
