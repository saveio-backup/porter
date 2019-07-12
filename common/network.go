/**
 * Description:
 * Author: Yihen.Liu
 * Create: 2019-05-05
 */
package common

import (
	"fmt"
	"github.com/saveio/themis/common/log"
	"net"
	"os"
	"strings"
)

func getDefaultIP() string {
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

func GetLocalIP() string {
	if Parameters.InterfaceName != "" && Parameters.InnerIP != "" {
		iface, err := net.InterfaceByName(Parameters.InterfaceName)
		if err != nil {
			log.Error("get interface err:", err.Error())
			os.Exit(1)
		}
		addrs, err := iface.Addrs()
		if err != nil {
			log.Error("get addrs err from special net interface:", err.Error())
			os.Exit(1)
		}
		for _, addr := range addrs {
			if strings.Split(addr.String(), "/")[0] == Parameters.InnerIP {
				return Parameters.InnerIP
			}
		}
		log.Error("inner IP is not match Interface, please review your config.json again.")
		os.Exit(1)
	}
	return getDefaultIP()
}

func GetPublicIP() string {
	if Parameters.PublicIP == "" {
		return GetLocalIP()
	}
	return Parameters.PublicIP
}

func GetPortFromParamsByProtocol(protocol string) int {
	switch protocol {
	case "udp":
		return Parameters.UPort
	case "kcp":
		return Parameters.KPort
	case "quic":
		return Parameters.QPort
	case "tcp":
		return Parameters.TPort
	default:
		log.Error("please use correct protocol, kcp or udp. not support:", protocol)
		return -1
	}
}

func GetPublicHost(protocol string) string {
	return fmt.Sprintf("%s://%s:%d", protocol, GetPublicIP(), GetPortFromParamsByProtocol(protocol))
}

func GetLogDir() string {
	return Parameters.LogDir
}
