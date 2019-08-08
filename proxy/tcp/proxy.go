/**
 * Description:
 * Author: Yihen.Liu
 * Create: 2019-05-30
 */
package tcp

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/saveio/porter/common"
	"github.com/saveio/porter/internal/protobuf"
	"github.com/saveio/themis/common/log"
)

func (p *TCPProxyServer) startListenScheduler() {
	for {
		select {
		case item := <-p.listenerBuffer:
			var proxyIP string
			if value, ok := p.proxies.Load(item.connectionID); !ok {
				proxyIP = p.proxyListenAndAccept(item.connectionID, item.state)
				if "" == proxyIP {
					log.Error("in startListenScheduler, listen and accept get nil proxyIP value")
					continue
				}
				if err := sendMessage(item.state, &protobuf.ProxyResponse{ProxyAddress: proxyIP}); err != nil {
					log.Error("tcp proxy handle listen scheduler err:", err.Error())
				}
			} else {
				log.Info(fmt.Sprintf("(tcp) origin (%s) relay ip is: %s, has exist.", item.connectionID, value.(peer).addr))
				if err := sendMessage(item.state, &protobuf.ProxyResponse{ProxyAddress: value.(peer).addr}); err != nil {
					log.Error("tcp proxy handle listen scheduler when re-sent, err:", err.Error())
				}
			}
		case <-p.stop:
			return
		}
	}
}

func (p *TCPProxyServer) proxyListenAndAccept(connectionID string, state *ConnState) string {
	port := common.RandomPort("tcp", connectionID)
	listener, err := listen(common.GetLocalIP(), port)
	if err != nil {
		log.Error("proxy listen server start ERROR:", err.Error())
		return ""
	} else {
		log.Infof("start proxyListen for inbound proxy apply(remote-ip:%s), proxy-ip/port:%s", state.conn.RemoteAddr(), listener.Addr().String())
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

func (p *TCPProxyServer) onceAccept(peerInfo peer, connectionID string) error {
	conn, err := peerInfo.listener.Accept()
	if err != nil {
		log.Error("peer proxy accept err:", err.Error(), ",listen ip:", peerInfo.listener.Addr().String())
		return err
	} else {
		log.Info("accept a new inbound connection to proxy server:", peerInfo.addr, "remote client addr:", conn.RemoteAddr().String())
	}
	go func(conn net.Conn) {
		defer conn.Close()
		//defer p.releasePeerResource(connectionID) //不要releasePeerResource， 只释放掉出问题的连接即可，不要释放无关连接；
		connState := newConnState(conn, "")
		close(connState.stop) //connState.stop没有使用，可以立刻关闭;
		for {
			select {
			case <-peerInfo.state.stop:
				log.Info("goroutine exit, listen ip is: ", peerInfo.addr)
				return
			default:
				buffer, err, sendFrom, opcode, nonce := receiveTcpRawMessage(connState, peerInfo.addr)
				if 0 == len(buffer) || nil == buffer {
					log.Error("(tcp) onceAccept groutine, receive empty message. proxy listen server/ip:", peerInfo.addr,
						"remote client addr:", conn.RemoteAddr().String())
					return
				}
				if err != nil {
					log.Error("(tcp) onceAccept goroutine receiveTcpRawMsg err:", err.Error(), "proxy listen server/ip:",
						peerInfo.addr, "remote client addr:", conn.RemoteAddr().String())
					return
				}
				err = transferTcpRawMessage(buffer, peerInfo.state)
				if err != nil {
					log.Error("transfer tcp raw message err:", err.Error(), "proxy listen server/ip addr:", peerInfo.addr,
						"remote client addr:", conn.RemoteAddr().String())
					return
				} else {
					log.Info("transfer tcp raw message success, sender from:", sendFrom, ",send to:", peerInfo.addr, ",msg.opcode:", opcode, ",msg.Nonce:", nonce)
				}
			}
		}
	}(conn)
	return nil
}

func (p *TCPProxyServer) proxyAccept(peerInfo peer, connectionID string) error {
	for {
		select {
		case <-peerInfo.stop:
			log.Info("peerInfo receive stop signal, exit now.")
			return nil
		default:
			if err := p.onceAccept(peerInfo, connectionID); err != nil {
				log.Error("proxyAccept run err when gothrough onceAccept, err:", err.Error())
				p.releasePeerResource(connectionID)
				return err
			}
		}
	}
	return nil
}
