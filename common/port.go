/**
 * Description:
 * Author: Yihen.Liu
 * Create: 2019-05-20 
*/
package common

import (
	"sync"
	"github.com/saveio/themis/common/log"
	"math/rand"
)

type protocols struct {
	udp bool
	kcp bool
	tcp bool
}

type Ports struct {
	usingPorts *sync.Map
	begin int
	ranges int
	writeMutex *sync.Mutex
}

var ports Ports

func RandomPort(protocol string) uint16 {
	ports.writeMutex.Lock()
	start:= rand.Intn(ports.ranges)
	for{
		port := uint16(start+ports.begin)
		if _, ok:= ports.usingPorts.Load(port); !ok {
			switch protocol {
			case "udp":
				ports.usingPorts.Store(port,protocols{udp:true})
			case "kcp":
				ports.usingPorts.Store(port,protocols{kcp:true})
			case "tcp":
				ports.usingPorts.Store(port,protocols{tcp:true})
			default:
				ports.writeMutex.Unlock()
				log.Error("not support ", protocol, ", please use tcp/kcp/udp.")
				return 0
			}
			ports.writeMutex.Unlock()
			return port
		}else{
			start += 1
		}
	}
}

func InitPorts()  {
	ports = Ports{
		begin:Parameters.RandomPortBegin,
		ranges:Parameters.RandomPortRange,
		usingPorts:new(sync.Map),
		writeMutex:new(sync.Mutex),
	}
}
