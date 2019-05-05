/**
 * Description:
 * Author: Yihen.Liu
 * Create: 2019-05-05 
*/
package common

import (
	"net"
	"fmt"
	"os"
)

func GetLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		fmt.Println(err)
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