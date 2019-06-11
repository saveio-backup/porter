/**
 * Description:
 * Author: Yihen.Liu
 * Create: 2019-05-09
 */
package quic

import (
	"encoding/hex"
	"github.com/saveio/porter/internal/protobuf"
	"github.com/saveio/porter/types/opcode"
	"github.com/saveio/themis/common/log"
	"time"
)

const writeFLushLatency = 50 * time.Millisecond

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

func (p *QuicProxyServer) releasePeerResource(ConnectionID string) {
	if peerInfo, ok := p.proxies.Load(ConnectionID); ok {
		close(peerInfo.(peer).stop)
		close(peerInfo.(peer).state.stop)
		peerInfo.(peer).conn.Close()
		peerInfo.(peer).state.conn.Close()
		p.proxies.Delete(ConnectionID)
	}
}

func (p *QuicProxyServer) handleProxyRequestMessage(message *protobuf.Message, state *ConnState) {

	//if the client is working in public-net environment, return ip address directly
/*	if message.Sender.Address == state.conn.RemoteAddr().String() {
		addrInfo, err := common.ParseAddress(state.conn.RemoteAddr().String())
		if err != nil {
			log.Error("parse remoteAddr err:", err.Error())
			return
		}
		sendMessage(state, &protobuf.ProxyResponse{ProxyAddress: addrInfo.ToString()})
		return
	}*/

	ConnectionID := hex.EncodeToString(message.Sender.ConnectionId)
	p.listenerBuffer <- peerListen{connectionID: ConnectionID, state: state}
	go flushLoop(state)
}

func (p *QuicProxyServer) handleProxyKeepaliveMessage(message *protobuf.Message, state *ConnState) {
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
		sendMessage(state, &protobuf.KeepaliveResponse{})
	}
}

func (p *QuicProxyServer) handleDisconnectMessage(message *protobuf.Message) {
	ConnectionID := hex.EncodeToString(message.Sender.ConnectionId)
	p.releasePeerResource(ConnectionID)
}

func (p *QuicProxyServer) handleControlMessage() {
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
