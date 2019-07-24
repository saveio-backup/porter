/**
 * Description:
 * Author: Yihen.Liu
 * Create: 2019-05-30
 */
package quic

import (
	"fmt"
	"github.com/lucas-clemente/quic-go"
	"github.com/saveio/porter/common"
	"github.com/saveio/porter/internal/protobuf"
	"github.com/saveio/themis/common/log"
	"sync"
	"time"
)

func (p *QuicProxyServer) startListenScheduler() {
	for {
		select {
		case item := <-p.listenerBuffer:
			var proxyIP string
			if value, ok := p.proxies.Load(item.connectionID); !ok {
				proxyIP = p.proxyListenAndAccept(item.connectionID, item.state)
				err := sendMessage(item.state, &protobuf.ProxyResponse{ProxyAddress: proxyIP})
				if err != nil {
					log.Error("quic listen scheduler err:", err.Error())
				}
			} else {
				log.Info(fmt.Sprintf("(quic) origin (%s) relay ip is: %s, has exist.", item.connectionID, value.(peer).addr))
				if err := sendMessage(item.state, &protobuf.ProxyResponse{ProxyAddress: value.(peer).addr}); err != nil {
					log.Error("quic proxy handle listen scheduler when re-sent, err:", err.Error())
				}
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
		log.Error("proxy listen server start ERROR:", err.Error())
		return ""
	} else {
		log.Info("listen to server", listener.Addr().String())
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
	conn, err := peerInfo.listener.Accept()
	if err != nil {
		log.Error("(quic) peer proxy accept err:", err.Error(), "listen ip", peerInfo.listener.Addr().String())
		return err
	}
	stream, err := conn.AcceptStream()
	go func(stream quic.Stream, conn quic.Session) {
		defer stream.Close()
		defer conn.Close()
		//defer p.releasePeerResource(connectionID) //不要releasePeerResource， 只释放掉出问题的连接即可，不要释放无关连接；
		connState := newConnState(stream, "")
		close(connState.stop) //connState.stop没有使用，可以立刻关闭;
		for {
			select {
			case <-peerInfo.state.stop:
				log.Info("(quic) goroutine exit, listen ip is ", peerInfo.listener.Addr().String())
				return
			default:
				buffer, err := receiveQuicRawMessage(connState)
				if 0 == len(buffer) || nil == buffer {
					log.Error("(quic) onceAccept groutine, receive empty message")
					return
				}
				err = transferQuicRawMessage(buffer, peerInfo.state)
				if err != nil {
					log.Error("(quic) transfer quic raw message err:", err.Error(), "addr:", peerInfo.listener.Addr().String())
					return
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
				return err
			}
		}
	}
	return nil
}
