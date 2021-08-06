/**
 * Description:
 * Author: Yihen.Liu
 * Create: 2019-04-26
 */
package quic

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"math/big"
	mRand "math/rand"
	"sync"
	"time"

	"os"
	"os/signal"
	"syscall"

	"github.com/lucas-clemente/quic-go"
	"github.com/saveio/porter/common"
	"github.com/saveio/porter/internal/protobuf"
	"github.com/saveio/porter/types/opcode"
	"github.com/saveio/themis/common/log"
)

const (
	MONITOR_TIME_INTERVAL   = 3
	PEER_MONITOR_TIMEOUT    = 10 * time.Second
	MESSAGE_CHANNEL_LEN     = 65535
	LISTEN_CHANNEL_LEN      = 65535
	DEFAULT_PORT_CACHE_TIME = 7200
)

type port struct {
	start uint32
	end   uint32
	used  uint32
}

type peer struct {
	addr       string
	conn       quic.Stream
	listener   quic.Listener
	loginTime  time.Time
	updateTime time.Time
	stop       chan struct{}
	state      *ConnState
	release    *sync.Once
}
type msgNotify struct {
	message *protobuf.Message
	state   *ConnState
}

type peerListen struct {
	connectionID string
	state        *ConnState
}

type QuicProxyServer struct {
	mainListener   quic.Listener
	proxies        *sync.Map
	ports          port
	msgBuffer      chan msgNotify
	listenerBuffer chan peerListen
	stop           chan struct{}
}

type ConnState struct {
	session      quic.Session
	conn         quic.Stream
	writer       *bufio.Writer
	messageNonce uint64
	writerMutex  *sync.Mutex
	stop         chan struct{}
	connectionID string
	remoteAddr   string
}

func init() {
	mRand.Seed(time.Now().UnixNano())
}

func Init() *QuicProxyServer {
	return &QuicProxyServer{
		proxies:        new(sync.Map),
		msgBuffer:      make(chan msgNotify, MESSAGE_CHANNEL_LEN),
		listenerBuffer: make(chan peerListen, LISTEN_CHANNEL_LEN),
		stop:           make(chan struct{}),
	}
}

func newConnState(listenr quic.Stream, addr string, session quic.Session) *ConnState {
	return &ConnState{
		session:      session,
		conn:         listenr,
		writer:       bufio.NewWriterSize(listenr, common.Parameters.WriteBufferSize),
		messageNonce: 0,
		writerMutex:  new(sync.Mutex),
		stop:         make(chan struct{}),
		remoteAddr:   addr,
	}
}

// Setup a bare-bones TLS config for the server
func generateTLSConfig() *tls.Config {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		panic(err)
	}
	template := x509.Certificate{SerialNumber: big.NewInt(1)}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		panic(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		panic(err)
	}
	return &tls.Config{Certificates: []tls.Certificate{tlsCert}, NextProtos: []string{"quic-proxy"}}
}

// Listen listens for incoming quic connections on a specified port.
func listen(ip string, port uint16) (quic.Listener, error) {
	resolved := fmt.Sprintf("%s:%d", ip, port)
	listener, err := quic.ListenAddr(resolved, generateTLSConfig(), &quic.Config{KeepAlive: true, MaxIdleTimeout: time.Second * 15})

	if err != nil {
		log.Error("quic listen start err:", err.Error())
		return nil, err
	}

	return listener, nil
}

func (p *QuicProxyServer) quicServerListenAndAccept(ip string, port uint16) {
	var err error
	p.mainListener, err = listen(ip, port)
	if err != nil {
		log.Errorf("quic server listen start ERROR:", err.Error())
		return
	} else {
		log.Info("Quic Proxy Listen IP:", p.mainListener.Addr().String())
	}
	go p.serverAccept()
	go p.handleControlMessage()
	go p.startListenScheduler()
}

func (p *QuicProxyServer) serverAccept() error {
	for {
		conn, err := p.mainListener.Accept(context.Background())
		if err != nil {
			log.Error("quic listener accept error:", err.Error(), "listen addr:", p.mainListener.Addr().String())
			continue
		}
		stream, err := conn.AcceptStream(context.Background())
		if err != nil {
			log.Error("quic accept stream err:", err.Error(), "listen addr:", p.mainListener.Addr().String())
			conn.CloseWithError(0, "")
			continue
		}
		go func(stream quic.Stream, conn quic.Session) {
			connState := newConnState(stream, conn.RemoteAddr().String(), conn)
			firstInboundMsg := true
			for {
				message, err := receiveMessage(connState)
				if nil == message || err != nil {
					log.Warn("quic receive message goroutine err:", err.Error(), "listen remote addr:", conn.RemoteAddr().String())
					if firstInboundMsg == true || connState.connectionID == "" {
						log.Error("first inbound quic message is error, connection will be closed immediately.")
						conn.CloseWithError(0, "")
						break
					}
					p.releasePeerResource(connState.connectionID)
					break
				}
				log.Info("receive a new quic message which need to be controll message type in main Accept, message.opcode:",
					message.Opcode, "netID:", message.NetID, "sender address:", message.Sender.Address)
				if message.Opcode == uint32(opcode.ProxyRequestCode) || message.Opcode == uint32(opcode.KeepaliveCode) {
					p.msgBuffer <- msgNotify{message: message, state: connState}
					firstInboundMsg = false
				}
			}
		}(stream, conn)
	}
	close(p.stop)
	return nil
}

func (p *QuicProxyServer) monitorPeerStatus() {
	interval := time.Tick(MONITOR_TIME_INTERVAL * time.Second)
	for {
		select {
		case <-interval:
			p.proxies.Range(func(key, value interface{}) bool {
				if time.Now().After(value.(peer).updateTime.Add(PEER_MONITOR_TIMEOUT)) {
					p.releasePeerResource(key.(string))
					log.Info("client has disconnect from proxy server as for monitor timeout, proxy-addr:", value.(peer).addr,
						",monitor timeout second:", PEER_MONITOR_TIMEOUT, ",lastst update time:", value.(peer).updateTime)
				}
				return true
			})

			timeout := common.Parameters.PortTimeout
			if common.Parameters.PortTimeout <= 0 {
				timeout = DEFAULT_PORT_CACHE_TIME
			}
			common.PortSet.Cache.Range(func(key, value interface{}) bool {
				if time.Now().After(value.(*common.UsingPort).Timestamp.Add(timeout * time.Second)) {
					delKey := fmt.Sprintf("%s-%s", value.(*common.UsingPort).Protocol, value.(*common.UsingPort).ConnectionID)
					common.PortSet.Cache.Delete(delKey)
					log.Info("proxy port timeout in memory cache:", delKey, " delete now. latest timestamp:", value.(*common.UsingPort).Timestamp)
				}
				return true
			})
		}
	}
}

func (p *QuicProxyServer) waitExit() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	select {
	case sig := <-sigs:
		log.Infof("QuicProxyServer received exit signal:%v,", sig.String(), ",begin to release all resource.")
		p.proxies.Range(func(key, value interface{}) bool {
			p.releasePeerResource(key.(string))
			return true
		})
		p.mainListener.Close()
		os.Exit(0)
	}
}

func (p *QuicProxyServer) StartQuicServer(port uint16) {
	go p.monitorPeerStatus()
	go p.quicServerListenAndAccept(common.GetLocalIP(), port)
	go p.waitExit()
	<-make(chan struct{})
}
