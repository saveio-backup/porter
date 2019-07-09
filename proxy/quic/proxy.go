/**
 * Description:
 * Author: Yihen.Liu
 * Create: 2019-05-30
 */
package quic

import (
	"fmt"
	"github.com/saveio/porter/common"
	"github.com/saveio/themis/common/log"
	"time"
	"github.com/saveio/porter/internal/protobuf"
	"github.com/lucas-clemente/quic-go"
)

func (p *QuicProxyServer) startListenScheduler() {
	for {
		select {
		case item := <-p.listenerBuffer:
			var proxyIP string

		if _, ok:=p.proxies.Load(item.connectionID);!ok{
			proxyIP = p.proxyListenAndAccept(item.connectionID, item.state)
			sendMessage(item.state, &protobuf.ProxyResponse{ProxyAddress: proxyIP})
		}else{
			log.Info(fmt.Sprintf("(quic) origin (%s) relay ip is: %s, has exist.", item.connectionID, proxyIP))
		}

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

func (p *QuicProxyServer) proxyListenAndAccept(connectionID string, state *ConnState) string {
	port := common.RandomPort("quic")
	listener, err := listen(common.GetLocalIP(), port)
	if err != nil {
		log.Error("proxy listen server start ERROR:", err.Error())
		return ""
	}else {
		log.Info("listen to server",listener.Addr().String())
	}

	peerInfo := peer{addr: fmt.Sprintf("quic://%s:%d", common.GetLocalIP(), port),
		state:      state,
		conn:       state.conn,
		listener:   listener,
		loginTime:  time.Now(),
		updateTime: time.Now(),
		stop:       make(chan struct{}),
	}
	p.proxies.Store(connectionID, peerInfo)

	go p.proxyAccept(peerInfo, connectionID)
	return fmt.Sprintf("%s:%d", common.GetPublicIP(), port)
}

func (p *QuicProxyServer) proxyAccept(peerInfo peer, connectionID string) error {
	for {
		conn, err := peerInfo.listener.Accept()
		if err != nil {
			log.Error("peer proxy accept err:", err.Error(), "listen ip", peerInfo.listener.Addr().String())
			continue
		}
		stream, err:= conn.AcceptStream()
		go func(stream quic.Stream, conn quic.Session) {
			defer stream.Close()
			defer conn.Close()
			//defer p.releasePeerResource(connectionID); 不要releasePeerResource， 只释放掉出问题的连接即可，不要释放无关连接；
			connState := newConnState(stream,"")
			close(connState.stop) //connState.stop没有使用，可以立刻关闭;
			for {
				select {
				case <-peerInfo.state.stop:
					log.Info("groutine exit, listen ip is ", peerInfo.listener.Addr().String())
					return
				default:
					buffer, _ := receiveQuicRawMessage(connState)
					err := transferQuicRawMessage(buffer, peerInfo.state)
					if err!=nil {
						log.Error("transfer quic raw message err:", err.Error(),"addr:", peerInfo.listener.Addr().String())
						return
					}
				}
			}
		}(stream, conn)
	}
	return nil
}
