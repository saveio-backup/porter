/**
 * Description:
 * Author: Yihen.Liu
 * Create: 2019-05-09 
*/
package proxy

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
	peerID := hex.EncodeToString(message.Sender.Id)

	peerInfo, ok:=p.proxies.Load(peerID)
	if !ok{
		proxyIP = p.proxyListenAndAccept(peerID, remoteAddr) //DialAddress应该是对方的公网IP, receiveUDPMessage的时候返回
		sendUDPMessage(&protobuf.ProxyResponse{ProxyAddress:proxyIP}, p.listener, remoteAddr)
	} else if peerInfo.(peer).addr != remoteAddr {
		p.proxies.Delete(peerID)
		proxyIP = p.proxyListenAndAccept(peerID, remoteAddr)
		sendUDPMessage(&protobuf.ProxyResponse{ProxyAddress:proxyIP}, p.listener, remoteAddr)
	}
}

func(p *ProxyServer) handleProxyKeepaliveMessage(message *protobuf.Message){
	peerID := hex.EncodeToString(message.Sender.Id)
	if peerInfo, ok := p.proxies.Load(peerID); ok{
		p.proxies.Delete(peerID)
		p.proxies.Store(peerID,
			peer{
			addr: 			peerInfo.(peer).addr,
			conn: 			peerInfo.(peer).conn,
			updateTime: 	time.Now(),
			loginTime:		peerInfo.(peer).loginTime,
		})
	}
}

func (p *ProxyServer) releasePeerResource(peerID string){
	if peerInfo, ok := p.proxies.Load(peerID); ok{
		//close(peerInfo.(peer).stop)
		peerInfo.(peer).conn.Close()
		p.proxies.Delete(peerID)
	}
}

func(p *ProxyServer) handleDisconnectMessage(message *protobuf.Message){
	peerID := hex.EncodeToString(message.Sender.Id)
	p.releasePeerResource(peerID)
}
