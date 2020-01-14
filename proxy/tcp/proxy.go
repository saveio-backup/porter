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

			if value, ok := p.proxies.Load(item.connectionID); ok {
				log.Info(fmt.Sprintf("(tcp) origin (%s) relay ip is: %s, has exist, don't delete relevant resource immediately but cover old value except for listen conn.", item.connectionID, value.(peer).addr))
				value.(peer).listener.Close()
				//p.Metric.ProxyCounter.Dec(1)
				//close(value.(peer).stop)
			}

			proxyIP = p.proxyListenAndAccept(item.connectionID, item.state)
			if "" == proxyIP {
				log.Error("in TCP startListenScheduler, listen and accept get nil proxyIP value")
				continue
			}
			if err := sendMessage(item.state, &protobuf.ProxyResponse{ProxyAddress: proxyIP}); err != nil {
				log.Error("tcp proxy handle listen scheduler err:", err.Error(), ",proxy ip:", proxyIP)
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
		log.Error("proxy listen server start ERROR:", err.Error(), "random port:", port)
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
	//p.Metric.ProxyCounter.Inc(1)
	return fmt.Sprintf("%s:%d", common.GetPublicIP(), port)
}

func (p *TCPProxyServer) proxyAccept(peerInfo peer, connectionID string) error {
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

func (p *TCPProxyServer) onceAccept(peerInfo peer, connectionID string) error {
	conn, err := peerInfo.listener.Accept()
	if err != nil {
		log.Error("peer proxy accept err:", err.Error(), ",listen ip:", peerInfo.listener.Addr().String())
		return err
	} else {
		//p.Metric.ProxyConnCounter.Inc(1)
		log.Info("accept a new inbound connection to proxy server:", peerInfo.addr, "remote client addr:", conn.RemoteAddr().String())
	}
	go func(conn net.Conn) {
		defer func() {
			conn.Close()
			//p.Metric.ProxyConnCounter.Dec(1)
		}()
		//defer p.releasePeerResource(connectionID) //不要releasePeerResource， 只释放掉出问题的连接即可，不要释放无关连接；
		connState := newConnState(conn, "")
		close(connState.stop) //connState.stop没有使用，可以立刻关闭;
		for {
			select {
			case <-peerInfo.stop:
				log.Info("tcp goroutine exit receive stop signal: peerInfo.stop, listen ip is: ", peerInfo.addr)
				return
			case <-peerInfo.state.stop:
				log.Info("tcp goroutine exit receive stop signal: peerInfo.state.stop, listen ip is: ", peerInfo.addr)
				return
			default:
				buffer, err := p.receiveTcpRawMessage(connState, peerInfo.addr)
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
				//start := time.Now()
				err = transferTcpRawMessage(buffer, peerInfo.state)
				if err != nil {
					log.Error("transfer tcp raw message err:", err.Error(), "proxy listen server/ip addr:", peerInfo.addr,
						"remote client addr:", conn.RemoteAddr().String())
					return
				} else {
					log.Info("transfer tcp raw message success, send to:", peerInfo.addr, "msg.len:", len(buffer))
					//p.Metric.TransferAmountGauge.Update(int64(len(buffer)))
					//p.Metric.SendRawTimeGauge.Update(int64(time.Since(start)))
					peerInfo.updateTime = time.Now()
				}
			}
		}
	}(conn)
	return nil
}
