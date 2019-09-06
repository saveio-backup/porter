/**
 * Description:
 * Author: Yihen.Liu
 * Create: 2019-05-30
 */
package quic

import (
	"fmt"
	"sync"
	"time"

	"context"

	"github.com/lucas-clemente/quic-go"
	"github.com/saveio/porter/common"
	"github.com/saveio/porter/internal/protobuf"
	"github.com/saveio/themis/common/log"
)

func (p *QuicProxyServer) startListenScheduler() {
	for {
		select {
		case item := <-p.listenerBuffer:
			var proxyIP string
			if value, ok := p.proxies.Load(item.connectionID); ok {
				log.Info(fmt.Sprintf("(quic) origin (%s) relay ip is: %s, has exist, don't delete relevant resource immediately but cover old value except for listener conn.", item.connectionID, value.(peer).addr))
				value.(peer).listener.Close()
				close(value.(peer).stop)
			}
			proxyIP = p.proxyListenAndAccept(item.connectionID, item.state)
			if "" == proxyIP {
				log.Error("in QUIC startListenScheduler, listen and accept get nil proxyIP value")
				continue
			}

			err := sendMessage(item.state, &protobuf.ProxyResponse{ProxyAddress: proxyIP})
			if err != nil {
				log.Error("quic listen scheduler err:", err.Error(), ",proxy ip:", proxyIP)
			}
		case <-p.stop:
			return
		}
	}
}

func (p *QuicProxyServer) proxyListenAndAccept(connectionID string, state *ConnState) string {
	port := common.RandomPort("quic", connectionID)
	listener, err := listen(common.GetLocalIP(), port)
	if err != nil {
		log.Error("proxy listen server start ERROR:", err.Error(), ",random port:", port)
		return ""
	} else {
		log.Infof("start proxyListen for inbound proxy apply(remote-ip:%s), proxy-ip/port:%s", state.remoteAddr, listener.Addr().String())
	}

	peerInfo := peer{addr: fmt.Sprintf("%s:%d", common.GetPublicIP(), port),
		state:      state,
		conn:       state.conn,
		listener:   listener,
		loginTime:  time.Now(),
		updateTime: time.Now(),
		stop:       make(chan struct{}),
		release:    new(sync.Once),
	}
	p.proxies.Store(connectionID, peerInfo)

	go p.proxyAccept(peerInfo, connectionID)
	return fmt.Sprintf("%s:%d", common.GetPublicIP(), port)
}

func (p *QuicProxyServer) onceAccept(peerInfo peer, connectionID string) error {
	conn, err := peerInfo.listener.Accept(context.Background())
	if err != nil {
		log.Error("(quic) peer proxy accept err:", err.Error(), "listen ip", peerInfo.listener.Addr().String())
		return err
	}
	stream, err := conn.AcceptStream(context.Background())
	go func(stream quic.Stream, conn quic.Session) {
		defer stream.Close()
		defer conn.Close()
		//defer p.releasePeerResource(connectionID) //不要releasePeerResource， 只释放掉出问题的连接即可，不要释放无关连接；
		connState := newConnState(stream, "", conn)
		close(connState.stop) //connState.stop没有使用，可以立刻关闭;
		for {
			select {
			case <-peerInfo.stop:
				log.Info("quic goroutine exit receive stop signal: peerInfo.stop, listen ip is: ", peerInfo.addr)
				return
			case <-peerInfo.state.stop:
				log.Info("quic goroutine exit receive stop signal: peerInfo.state.stop, listen ip is: ", peerInfo.addr)
				return
			default:
				buffer, err := receiveQuicRawMessage(connState, peerInfo.addr)
				if 0 == len(buffer) || nil == buffer {
					log.Error("(quic) onceAccept groutine, receive empty message. proxy listen server/ip:", peerInfo.addr,
						"remote client addr:", conn.RemoteAddr().String())
					return
				}
				if err != nil {
					log.Error("(quic) onceAccept goroutine receiveTcpRawMsg err:", err.Error(), "proxy listen server/ip:",
						peerInfo.addr, "remote client addr:", conn.RemoteAddr().String())
					return
				}
				err = transferQuicRawMessage(buffer, peerInfo.state)
				if err != nil {
					log.Error("transfer quic raw message err:", err.Error(), "proxy listen server/ip addr:", peerInfo.addr,
						"remote client addr:", conn.RemoteAddr().String())
					return
				} else {
					log.Info("transfer quic raw message success, send to:", peerInfo.addr, "msg.len:", len(buffer))
				}
			}
		}
	}(stream, conn)
	return nil
}

func (p *QuicProxyServer) proxyAccept(peerInfo peer, connectionID string) error {
	for {
		select {
		case <-peerInfo.stop:
			log.Info("peerInfo receive stop signal, exit now.")
			return nil
		default:
			if err := p.onceAccept(peerInfo, connectionID); err != nil {
				log.Error("proxyAccept run err when gothrough onceAccept, err:", err.Error(), ",proxy-addr:", peerInfo.addr)
				//p.releasePeerResource(connectionID)
				return err
			}
		}
	}
	return nil
}
