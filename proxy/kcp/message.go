/**
 * Description:
 * Author: Yihen.Liu
 * Create: 2019-05-09 
*/
package kcp

import (
	"github.com/saveio/porter/internal/protobuf"
	"encoding/hex"
	"github.com/saveio/themis/common/log"
	"github.com/saveio/porter/common"
	"time"
	"net"
	"fmt"
)

func(p *KcpProxyServer) handleProxyRequestMessage(message *protobuf.Message, conn net.Conn){

	//if the client is working in public-net environment, return ip address directly
	if message.Sender.Address == conn.RemoteAddr().String() {
		addrInfo, err:=common.ParseAddress(conn.RemoteAddr().String())
		if err!=nil{
			log.Error("parse remoteAddr err:", err.Error())
			return
		}
		sendMessage(conn, &protobuf.ProxyResponse{ProxyAddress:addrInfo.ToString()})
		return
	}
	fmt.Println("message.Sender.Address:", message.Sender.Address)
	fmt.Println("remote addr:", conn.RemoteAddr().String())
	var proxyIP string
	peerID := hex.EncodeToString(message.Sender.Id)

	peerInfo, ok:=p.proxies.Load(peerID)
	if !ok{
		proxyIP = p.proxyListenAndAccept(peerID, conn)
		sendMessage(conn, &protobuf.ProxyResponse{ProxyAddress:proxyIP})
	} else if peerInfo.(peer).addr != conn.RemoteAddr().String() {
		p.proxies.Delete(peerID)
		proxyIP = p.proxyListenAndAccept(peerID, conn)
		sendMessage(conn, &protobuf.ProxyResponse{ProxyAddress:proxyIP})
	}
}

func(p *KcpProxyServer) handleProxyKeepaliveMessage(message *protobuf.Message){
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

func (p *KcpProxyServer) releasePeerResource(peerID string){
	if peerInfo, ok := p.proxies.Load(peerID); ok{
		//close(peerInfo.(peer).stop)
		peerInfo.(peer).conn.Close()
		p.proxies.Delete(peerID)
	}
}

func(p *KcpProxyServer) handleDisconnectMessage(message *protobuf.Message){
	peerID := hex.EncodeToString(message.Sender.Id)
	p.releasePeerResource(peerID)
}
