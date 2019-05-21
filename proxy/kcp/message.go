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
	"fmt"
)
const writeFLushLatency = 50 * time.Millisecond

func flushLoop(state *ConnState) {
	t := time.NewTicker(writeFLushLatency)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			if err := state.writer.Flush(); err != nil {
				log.Errorf("flush err: %+v", err)
			}
		}
	}
}

func(p *KcpProxyServer) handleProxyRequestMessage(message *protobuf.Message, state *ConnState){

	//if the client is working in public-net environment, return ip address directly
	if message.Sender.Address == state.conn.RemoteAddr().String() {
		addrInfo, err:=common.ParseAddress(state.conn.RemoteAddr().String())
		if err!=nil{
			log.Error("parse remoteAddr err:", err.Error())
			return
		}
		sendMessage(state, &protobuf.ProxyResponse{ProxyAddress:addrInfo.ToString()})
		return
	}

	var proxyIP string
	peerID := hex.EncodeToString(message.Sender.Id)

	peerInfo, ok:=p.proxies.Load(peerID)
	if !ok{
		proxyIP = p.proxyListenAndAccept(peerID, state.conn)
		log.Info(fmt.Sprintf("origin (%s) relay ip is: %s", message.Sender.Address, proxyIP))
		sendMessage(state, &protobuf.ProxyResponse{ProxyAddress:proxyIP})
	} else if peerInfo.(peer).addr != state.conn.RemoteAddr().String() {
		p.proxies.Delete(peerID)
		proxyIP = p.proxyListenAndAccept(peerID, state.conn)
		sendMessage(state, &protobuf.ProxyResponse{ProxyAddress:proxyIP})
	}
	//go flushLoop(state)
}

func(p *KcpProxyServer) handleProxyKeepaliveMessage(message *protobuf.Message, state *ConnState){
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
		sendMessage(state, &protobuf.KeepaliveResponse{})
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
	fmt.Println("peer disconnect...")
	p.releasePeerResource(peerID)
}
