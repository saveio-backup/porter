/**
 * Description:
 * Author: Yihen.Liu
 * Create: 2019-05-05 
*/
package common

import (
	"net"
	"os"
	"github.com/saveio/themis/common/log"
)

func GetLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		log.Error("get local ip err:", err.Error())
		os.Exit(1)
	}

	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}