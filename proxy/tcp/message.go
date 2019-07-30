/**
 * Description:
 * Author: Yihen.Liu
 * Create: 2019-05-09
 */
package tcp

import (
	"encoding/hex"
	"fmt"
	"time"

	"github.com/saveio/porter/common"
	"github.com/saveio/porter/internal/protobuf"
	"github.com/saveio/porter/types/opcode"
	"github.com/saveio/themis/common/log"
)

const writeFLushLatency = 50 * time.Millisecond

func flushOnce(state *ConnState) error {
	state.writerMutex.Lock()
	if err := state.writer.Flush(); err != nil {
		log.Errorf("tcp flush err: %+v", err)
		state.writerMutex.Unlock()
		return err
	}
	state.writerMutex.Unlock()
	return nil
}

func flushLoop(state *ConnState) {
	t := time.NewTicker(writeFLushLatency)
	defer t.Stop()
	for {
		select {
		case <-state.stop:
			log.Info("tcp flush loop receive stop signal.")
			flushOnce(state)
			return
		case <-t.C:
			err := flushOnce(state)
			if err != nil {
				log.Error("(tcp) flush loop err:", err.Error())
				return
			}
		}
	}
}

func (p *TCPProxyServer) releasePeerResource(ConnectionID string) {
	if peerInfo, ok := p.proxies.Load(ConnectionID); ok {
		peerInfo.(peer).release.Do(func() {
			log.Info("release peer resource, proxy-ip:", peerInfo.(peer).addr)
			close(peerInfo.(peer).stop)
			close(peerInfo.(peer).state.stop)
			peerInfo.(peer).conn.Close()
			peerInfo.(peer).listener.Close()
			peerInfo.(peer).state.conn.Close()
			p.proxies.Delete(ConnectionID)
		})
	}
}

func (p *TCPProxyServer) handleProxyRequestMessage(message *protobuf.Message, state *ConnState) {

	//todo:if the client is working in public-net environment, return ip address directly
	ConnectionID := hex.EncodeToString(message.Sender.ConnectionId)
	state.connectionID = ConnectionID
	p.listenerBuffer <- peerListen{connectionID: ConnectionID, state: state}
	//go flushLoop(state)
}

func (p *TCPProxyServer) handleProxyKeepaliveMessage(message *protobuf.Message, state *ConnState) {
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
				listener:   peerInfo.(peer).listener,
				release:    peerInfo.(peer).release,
			})
		if err := sendMessage(state, &protobuf.KeepaliveResponse{}); err != nil {
			log.Error("tcp handleProxyKeepaliveMessage err:", err.Error())
		}
	}
	common.PortSet.WriteMutex.Lock()
	key := fmt.Sprintf("tcp-%s", ConnectionID)
	if port, ok := common.PortSet.Cache.Load(key); ok {
		port.(*common.UsingPort).Timestamp = time.Now()
		common.PortSet.WriteMutex.Unlock()
	}
}

func (p *TCPProxyServer) handleDisconnectMessage(message *protobuf.Message) {
	ConnectionID := hex.EncodeToString(message.Sender.ConnectionId)
	p.releasePeerResource(ConnectionID)
}

func (p *TCPProxyServer) handleControlMessage() {
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
			default:
				//log.Warn("please send correct control message type, include ProxyRequest/Keepalive/Disconnect, message.opcode:", item.message.Opcode, "send to ip:",item.message.Sender.Address)
			}
		case <-p.stop:
			return
		}
	}
}
