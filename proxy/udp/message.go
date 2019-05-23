/**
 * Description:
 * Author: Yihen.Liu
 * Create: 2019-05-09 
*/
package udp

import (
	"github.com/saveio/porter/internal/protobuf"
	"encoding/hex"
	"github.com/saveio/themis/common/log"
	"github.com/saveio/porter/common"
	"time"
)

func(p *ProxyServer) handleProxyRequestMessage(message *protobuf.Message, remoteAddr string){

	//if the client is working in public-net environment, return ip address directly
	if message.Sender.Address == remoteAddr{
		addrInfo, err:=common.ParseAddress(remoteAddr)
		if err!=nil{
			log.Error("parse remoteAddr err:", err.Error())
			return
		}
		sendUDPMessage(&protobuf.ProxyResponse{ProxyAddress:addrInfo.ToString()}, p.listener, remoteAddr)
		return
	}

	var proxyIP string
	ConnectionID := hex.EncodeToString(message.Sender.ConnectionId)

	peerInfo, ok:=p.proxies.Load(ConnectionID)
	if !ok{
		proxyIP = p.proxyListenAndAccept(ConnectionID, remoteAddr) //DialAddress应该是对方的公网IP, receiveUDPMessage的时候返回
		sendUDPMessage(&protobuf.ProxyResponse{ProxyAddress:proxyIP}, p.listener, remoteAddr)
	} else if peerInfo.(peer).addr != remoteAddr {
		p.proxies.Delete(ConnectionID)
		proxyIP = p.proxyListenAndAccept(ConnectionID, remoteAddr)
		sendUDPMessage(&protobuf.ProxyResponse{ProxyAddress:proxyIP}, p.listener, remoteAddr)
	}
}

func(p *ProxyServer) handleProxyKeepaliveMessage(message *protobuf.Message){
	ConnectionID := hex.EncodeToString(message.Sender.ConnectionId)
	if peerInfo, ok := p.proxies.Load(ConnectionID); ok{
		p.proxies.Delete(ConnectionID)
		p.proxies.Store(ConnectionID,
			peer{
			addr: 			peerInfo.(peer).addr,
			conn: 			peerInfo.(peer).conn,
			updateTime: 	time.Now(),
			loginTime:		peerInfo.(peer).loginTime,
		})
		sendUDPMessage(&protobuf.KeepaliveResponse{},p.listener, peerInfo.(peer).addr)
	}
}

func (p *ProxyServer) releasePeerResource(ConnectionID string){
	if peerInfo, ok := p.proxies.Load(ConnectionID); ok{
		//close(peerInfo.(peer).stop)
		peerInfo.(peer).conn.Close()
		p.proxies.Delete(ConnectionID)
	}
}

func(p *ProxyServer) handleDisconnectMessage(message *protobuf.Message){
	ConnectionID := hex.EncodeToString(message.Sender.ConnectionId)
	p.releasePeerResource(ConnectionID)
}
