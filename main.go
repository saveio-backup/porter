/**
 * Description:
 * Author: Yihen.Liu
 * Create: 2019-04-26 
*/
package main

import (
	udpProxy "github.com/saveio/porter/proxy/udp"
	kcpProxy "github.com/saveio/porter/proxy/kcp"
	"flag"
	"github.com/saveio/porter/common"
)

func main() {
	protocol:= flag.String("protocol", "", "protocol to use (kcp/tcp/udp)")
	flag.Parse()
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