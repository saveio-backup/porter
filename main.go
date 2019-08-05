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
	quicProxy "github.com/saveio/porter/proxy/quic"
	tcpProxy "github.com/saveio/porter/proxy/tcp"
	udpProxy "github.com/saveio/porter/proxy/udp"
	"github.com/saveio/themis/common/log"
)

func main() {
	protocol := flag.String("protocol", "", "protocol to use (kcp/tcp/udp)")
	flag.Parse()

	log.InitLog(common.Parameters.LogLevel, common.GetLogDir(), log.Stdout)
	go common.CheckLogFileSize()

	log.Debug("porter version:", common.Version)

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
