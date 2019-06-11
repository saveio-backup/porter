/**
 * Description:
 * Author: Yihen.Liu
 * Create: 2019-05-30
 */
package quic

import (
	"fmt"
	"github.com/saveio/porter/common"
	"github.com/saveio/porter/internal/protobuf"
	"github.com/saveio/themis/common/log"
	"time"
)

func (p *QuicProxyServer) startListenScheduler() {
	for {
		select {
		case item := <-p.listenerBuffer:
			var proxyIP string

			proxyIP = p.proxyListenAndAccept(item.connectionID, item.state)
			log.Info(fmt.Sprintf("origin (%s) relay ip is: %s", item.connectionID, proxyIP))
			sendMessage(item.state, &protobuf.ProxyResponse{ProxyAddress: proxyIP})

/*			peerInfo, ok := p.proxies.Load(item.connectionID)
			if !ok {
				proxyIP = p.proxyListenAndAccept(item.connectionID, item.state)
				//log.Info(fmt.Sprintf("origin (%s) relay ip is: %s", message.Sender.Address, proxyIP))
				sendMessage(item.state, &protobuf.ProxyResponse{ProxyAddress: proxyIP})
			} else if peerInfo.(peer).addr != item.state.conn.RemoteAddr().String() {
				p.proxies.Delete(item.connectionID)
				proxyIP = p.proxyListenAndAccept(item.connectionID, item.state)
				sendMessage(item.state, &protobuf.ProxyResponse{ProxyAddress: proxyIP})
			}*/
		}
	}
}

func (p *QuicProxyServer) proxyListenAndAccept(ConnectionID string, state *ConnState) string {
	port := common.RandomPort("quic")
	listener, err := listen(common.GetLocalIP(), port)
	if err != nil {
		log.Error("proxy listen server start ERROR:", err.Error())
		return ""
	}

	peerInfo := peer{addr: fmt.Sprintf("quic://%s:%d", common.GetLocalIP(), port),
		state:      state,
		conn:       state.conn,
		listener:   listener,
		loginTime:  time.Now(),
		updateTime: time.Now(),
		stop:       make(chan struct{}),
	}
	p.proxies.Store(ConnectionID, peerInfo)

	go p.proxyAccept(peerInfo)
	return fmt.Sprintf("%s:%d", common.GetPublicIP(), port)
}

func (p *QuicProxyServer) proxyAccept(peerInfo peer) error {
	for {
		conn, err := peerInfo.listener.Accept()
		if err != nil {
			log.Error("peer proxy accept err:", err.Error())
			continue
		}
		stream, err:= conn.AcceptStream()
		go func() {
			defer conn.Close()
			defer stream.Close()
			connState := newConnState(stream)
			close(connState.stop) //connState.stop没有使用，可以立刻关闭;
			for {
				buffer, _ := receiveQuicRawMessage(connState)
				transferQuicRawMessage(buffer, peerInfo.state)
			}
		}()
	}
	return nil
}
