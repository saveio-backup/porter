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
	flag.Parse()

	log.InitLog(common.Parameters.LogLevel, common.GetLogDir(), log.Stdout)
	go common.CheckLogFileSize()

	log.Debug("porter version:", common.Version)
	common.StartMonitor()
	switch common.GetProtocol() {
	case "udp":
		udpProxy.Init().StartUDPServer(uint16(common.GetPortFromParamsByProtocol("udp")))
	case "kcp":
		kcpProxy.Init().StartKCPServer(uint16(common.GetPortFromParamsByProtocol("kcp")))
	case "quic":
		quicProxy.Init().StartQuicServer(uint16(common.GetPortFromParamsByProtocol("quic")))
	case "tcp":
		tcpProxy.Init().StartTCPServer(uint16(common.GetPortFromParamsByProtocol("tcp")))
	default:
		go udpProxy.Init().StartUDPServer(uint16(common.GetPortFromParamsByProtocol("udp")))
		go kcpProxy.Init().StartKCPServer(uint16(common.GetPortFromParamsByProtocol("kcp")))
		go quicProxy.Init().StartQuicServer(uint16(common.GetPortFromParamsByProtocol("quic")))
		go tcpProxy.Init().StartTCPServer(uint16(common.GetPortFromParamsByProtocol("tcp")))
	}
	select {}
}
