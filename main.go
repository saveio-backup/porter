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
	default:
		udpProxy.Init().StartUDPServer(uint16(common.GetPortFromParamsByProtocol("udp")))
		kcpProxy.Init().StartKCPServer(uint16(common.GetPortFromParamsByProtocol("kcp")))
	}
	select {}
}
