/**
 * Description:
 * Author: Yihen.Liu
 * Create: 2019-05-30
 */
package tcp

import (
	"fmt"
	"github.com/saveio/porter/common"
	"github.com/saveio/themis/common/log"
	"time"
	"github.com/saveio/porter/internal/protobuf"
	"net"
	"sync"
)

func (p *TCPProxyServer) startListenScheduler() {
	for {
		select {
		case item := <-p.listenerBuffer:
			var proxyIP string
			if _, ok:=p.proxies.Load(item.connectionID);!ok{
				proxyIP = p.proxyListenAndAccept(item.connectionID, item.state)
				if err:= sendMessage(item.state, &protobuf.ProxyResponse{ProxyAddress: proxyIP});err!=nil{
					log.Error("tcp proxy handle listen scheduler err:",err.Error())
					if _,ok:=<-item.state.stop; ok{
						close(item.state.stop)
					}
				}
			}else{
				log.Info(fmt.Sprintf("(tcp) origin (%s) relay ip is: %s, has exist.", item.connectionID, proxyIP))
			}
		case <-p.stop:
			return
		}
	}
}

func (p *TCPProxyServer) proxyListenAndAccept(connectionID string, state *ConnState) string {
	port := common.RandomPort("tcp",connectionID)
	listener, err := listen(common.GetLocalIP(), port)
	if err != nil {
		log.Error("proxy listen server start ERROR:", err.Error())
		return ""
	}else {
		log.Info("listen to server",listener.Addr().String())
	}

	peerInfo := peer{addr: fmt.Sprintf("tcp://%s:%d", common.GetLocalIP(), port),
		state:      state,
		conn:       state.conn,
		listener:   listener,
		loginTime:  time.Now(),
		updateTime: time.Now(),
		stop:       make(chan struct{}),
		release:  	new(sync.Once),
	}
	p.proxies.Store(connectionID, peerInfo)

	go p.proxyAccept(peerInfo, connectionID)
	return fmt.Sprintf("%s:%d", common.GetPublicIP(), port)
}

func(p *TCPProxyServer) onceAccept(peerInfo peer, connectionID string) error{
	conn, err := peerInfo.listener.Accept()
	if err != nil {
		log.Error("peer proxy accept err:", err.Error(), "listen ip", peerInfo.listener.Addr().String())
		return err
	}
	go func(conn net.Conn) {
		defer conn.Close()
		//defer p.releasePeerResource(connectionID) //不要releasePeerResource， 只释放掉出问题的连接即可，不要释放无关连接；
		connState := newConnState(conn,"")
		close(connState.stop) //connState.stop没有使用，可以立刻关闭;
		for {
			select {
			case <-peerInfo.state.stop:
				log.Info("goroutine exit, listen ip is ", peerInfo.listener.Addr().String())
				return
			default:
				buffer, err := receiveTcpRawMessage(connState)
				if 0 == len(buffer) || nil == buffer{
					log.Error("(tcp) onceAccept groutine, receive empty message")
					return
				}
				err = transferTcpRawMessage(buffer, peerInfo.state)
				if err!=nil {
					log.Error("transfer tcp raw message err:", err.Error(),"addr:", peerInfo.listener.Addr().String())
					return
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
			if err:= p.onceAccept(peerInfo, connectionID); err!=nil{
				return err
			}
		}
	}
	return nil
}
