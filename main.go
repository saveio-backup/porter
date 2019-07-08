/**
 * Description:
 * Author: Yihen.Liu
 * Create: 2019-04-26
 */
package main

import (
	"flag"
	"github.com/saveio/porter/common"
	kcpProxy "github.com/saveio/porter/proxy/kcp"
	udpProxy "github.com/saveio/porter/proxy/udp"
	"github.com/saveio/themis/common/log"
	"os"
	quicProxy "github.com/saveio/porter/proxy/quic"
	tcpProxy "github.com/saveio/porter/proxy/tcp"
)

func main() {
	protocol := flag.String("protocol", "", "protocol to use (kcp/tcp/udp)")
	flag.Parse()
	log.InitLog(log.InfoLog,common.GetLogDir(),os.Stdout)
	switch *protocol {
	case "udp":
		udpProxy.Init().StartUDPServer(uint16(common.GetPortFromParamsByProtocol("udp")))
	case "kcp":
		kcpProxy.Init().StartKCPServer(uint16(common.GetPortFromParamsByProtocol("kcp")))
	case "quic":
		quicProxy.Init().StartQuicServer(uint16(common.GetPortFromParamsByProtocol("quic")))
	case "tcp":
		tcpProxy.Init().StartTCPServer(uint16(common.GetPortFromParamsByProtocol("tcp")))
	default:
		udpProxy.Init().StartUDPServer(uint16(common.GetPortFromParamsByProtocol("udp")))
		kcpProxy.Init().StartKCPServer(uint16(common.GetPortFromParamsByProtocol("kcp")))
		quicProxy.Init().StartQuicServer(uint16(common.GetPortFromParamsByProtocol("quic")))
		tcpProxy.Init().StartTCPServer(uint16(common.GetPortFromParamsByProtocol("tcp")))
	}
	select {}
}
