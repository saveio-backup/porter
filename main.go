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
	protocol:= flag.String("protocol", "", "protocol to use (kcp/tcp/udp)")
	kport := flag.Uint("kport", 6009, "port for kcp protocol to use")
	uport := flag.Uint("uport", 6008, "port for udp protocol to use")
	flag.Parse()
	switch *protocol {
	case "udp":
		udpProxy.Init().StartUDPServer(uint16(*uport))
	case "kcp":
		kcpProxy.Init().StartKCPServer(uint16(*kport))
	default:
		udpProxy.Init().StartUDPServer(uint16(*uport))
		kcpProxy.Init().StartKCPServer(uint16(*kport))
	}
	select {}
}