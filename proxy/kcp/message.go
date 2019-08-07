/**
 * Description:
 * Author: Yihen.Liu
 * Create: 2019-05-09
 */
package kcp

import (
	"encoding/hex"
	"fmt"
	"time"

	"github.com/saveio/porter/common"
	"github.com/saveio/porter/internal/protobuf"
	"github.com/saveio/porter/types/opcode"
	"github.com/saveio/themis/common/log"
)

const writeFLushLatency = 10 * time.Millisecond

func flushLoop(state *ConnState) {
	t := time.NewTicker(writeFLushLatency)
	defer t.Stop()
	for {
		select {
		case <-state.stop:
			return
		case <-t.C:
			state.writerMutex.Lock()
			if err := state.writer.Flush(); err != nil {
				log.Errorf("flush err: %+v", err)
			}
			state.writerMutex.Unlock()
		}
	}
}

func (p *KcpProxyServer) releasePeerResource(ConnectionID string) {
	if peerInfo, ok := p.proxies.Load(ConnectionID); ok {
		close(peerInfo.(peer).stop)
		close(peerInfo.(peer).state.stop)
		peerInfo.(peer).conn.Close()
		peerInfo.(peer).state.conn.Close()
		p.proxies.Delete(ConnectionID)
		log.Info("release proxy service for connectionID:", ConnectionID)
	}
}

func (p *KcpProxyServer) handleProxyRequestMessage(message *protobuf.Message, state *ConnState) {

	//if the client is working in public-net environment, return ip address directly
	if message.Sender.Address == state.conn.RemoteAddr().String() {
		addrInfo, err := common.ParseAddress(state.conn.RemoteAddr().String())
		if err != nil {
			log.Error("parse remoteAddr err:", err.Error())
			return
		}
		log.Infof("remote node is working in public-net env, return ip address directly. addr:%s", message.Sender.Address)
		sendMessage(state, &protobuf.ProxyResponse{ProxyAddress: addrInfo.ToString()})
		return
	}

	ConnectionID := hex.EncodeToString(message.Sender.ConnectionId)
	p.listenerBuffer <- peerListen{connectionID: ConnectionID, state: state}
	go flushLoop(state)
}

func (p *KcpProxyServer) handleProxyKeepaliveMessage(message *protobuf.Message, state *ConnState) {
	ConnectionID := hex.EncodeToString(message.Sender.ConnectionId)
	if peerInfo, ok := p.proxies.Load(ConnectionID); ok {
		p.proxies.Delete(ConnectionID)
		p.proxies.Store(ConnectionID,
			peer{
				addr:       peerInfo.(peer).addr,
				conn:       peerInfo.(peer).conn,
				updateTime: time.Now(),
				loginTime:  peerInfo.(peer).loginTime,
				stop:       peerInfo.(peer).stop,
				state:      peerInfo.(peer).state,
			})
		err := sendMessage(state, &protobuf.KeepaliveResponse{})
		if err != nil {
			log.Error("(quic) handle proxyKeepaliveMessage when send, error:", err.Error())
		}
	}
	common.PortSet.WriteMutex.Lock()
	key := fmt.Sprintf("kcp-%s", ConnectionID)
	if port, ok := common.PortSet.Cache.Load(key); ok {
		port.(*common.UsingPort).Timestamp = time.Now()
	}
	common.PortSet.WriteMutex.Unlock()
}

func (p *KcpProxyServer) handleDisconnectMessage(message *protobuf.Message) {
	ConnectionID := hex.EncodeToString(message.Sender.ConnectionId)
	log.Info("kcp receive disconnect signal ,connectionID:", ConnectionID)
	p.releasePeerResource(ConnectionID)
}

func (p *KcpProxyServer) handleControlMessage() {
	for {
		select {
		case item := <-p.msgBuffer:
			switch item.message.Opcode {
			case uint32(opcode.ProxyRequestCode):
				p.handleProxyRequestMessage(item.message, item.state)
			case uint32(opcode.KeepaliveCode):
				p.handleProxyKeepaliveMessage(item.message, item.state)
			case uint32(opcode.DisconnectCode):
				p.handleDisconnectMessage(item.message)
			}
		}
	}
}
