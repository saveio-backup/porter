/**
 * Description:
 * Author: Yihen.Liu
 * Create: 2019-04-26
 */
package quic

import (
	"bufio"
	"fmt"
	"github.com/saveio/porter/common"
	"github.com/saveio/porter/internal/protobuf"
	"github.com/saveio/themis/common/log"
	"crypto/rand"
	"sync"
	"time"
	"github.com/lucas-clemente/quic-go"
	"crypto/tls"
	"crypto/rsa"
	"crypto/x509"
	"math/big"
	"encoding/pem"
	mRand "math/rand"
	"github.com/saveio/porter/types/opcode"
)

const (
	MONITOR_TIME_INTERVAL = 3
	PEER_MONITOR_TIMEOUT  = 10 * time.Second
	MESSAGE_CHANNEL_LEN   = 65535
	LISTEN_CHANNEL_LEN    = 65535
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
	stop 			chan struct{}
}

type ConnState struct {
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
		stop:			make(chan struct{}),
	}
}

func newConnState(listenr quic.Stream, addr string) *ConnState {
	return &ConnState{
		conn:         listenr,
		writer:       bufio.NewWriterSize(listenr, defaultRecvBufferSize),
		messageNonce: 0,
		writerMutex:  new(sync.Mutex),
		stop:         make(chan struct{}),
		remoteAddr:	  addr,
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
	return &tls.Config{Certificates: []tls.Certificate{tlsCert}, NextProtos:[]string{"quic-proxy"}}
}

// Listen listens for incoming quic connections on a specified port.
func listen(ip string, port uint16) (quic.Listener, error) {
	resolved := fmt.Sprintf("%s:%d", ip, port)
	listener, err := quic.ListenAddr(resolved, generateTLSConfig(), &quic.Config{KeepAlive: true, IdleTimeout: time.Second * 15})

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
		conn, err := p.mainListener.Accept()
		if err != nil {
			log.Error("quic listener accept error:", err.Error(), "listen addr:", p.mainListener.Addr().String())
			continue
		}
		stream, err:= conn.AcceptStream()
		if err!=nil{
			log.Error("quic accept stream err:", err.Error(), "listen addr:",p.mainListener.Addr().String())
			conn.Close()
			continue
		}
		go func(stream quic.Stream, conn quic.Session) {
			connState := newConnState(stream,conn.RemoteAddr().String())
			for {
				message, err := receiveMessage(connState)
				if nil == message || err != nil {
					log.Error("quic receive message goroutine err:", err.Error(), "listen remote addr:",conn.RemoteAddr().String())
					p.releasePeerResource(connState.connectionID)
					break
				}
				if message.Opcode == uint32(opcode.ProxyRequestCode) || message.Opcode == uint32(opcode.KeepaliveCode){
					p.msgBuffer <- msgNotify{message: message, state: connState}
				}
			}
		}(stream,conn)
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
					log.Info("client has disconnect from proxy server, peerID:", key.(string))
				}
				return true
			})

			timeout:=common.Parameters.PortTimeout
			if common.Parameters.PortTimeout<=0{
				timeout = DEFAULT_PORT_CACHE_TIME
			}
			common.PortSet.Cache.Range(func(key, value interface{}) bool {
				if time.Now().After(value.(*common.UsingPort).Timestamp.Add(timeout*time.Second)){
					common.PortSet.Cache.Delete(fmt.Sprintf("%s-%s",value.(*common.UsingPort).Protocol,value.(*common.UsingPort).ConnectionID))
				}
				return true
			})
		}
	}
}

func (p *QuicProxyServer) StartQuicServer(port uint16) {
	go p.monitorPeerStatus()
	go p.quicServerListenAndAccept(common.GetLocalIP(), port)
	<- make(chan struct{})
}
