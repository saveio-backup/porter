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
)

func main() {
	protocolFlag := flag.String("protocol", "", "protocol to use (kcp/tcp/udp)")
	flag.Parse()
	protocol := *protocolFlag
	switch protocol {
	case "udp":
		udpProxy.Init().StartUDPServer()
	case "kcp":
		kcpProxy.Init().StartKCPServer()
	default:
		udpProxy.Init().StartUDPServer()
		kcpProxy.Init().StartKCPServer()
	}
	select {}
}