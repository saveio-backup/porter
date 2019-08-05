/**
 * Description:
 * Author: Yihen.Liu
 * Create: 2019-05-20
 */
package common

import (
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/saveio/themis/common/log"
)

const DEFAULT_PORT_CACHE_TIME = 7200

type protocols struct {
	udp  bool
	kcp  bool
	tcp  bool
	quic bool
}

type UsingPort struct {
	Timestamp    time.Time
	ConnectionID string
	Port         uint16
	Protocol     string
}

type Ports struct {
	usingPorts *sync.Map
	begin      int
	ranges     int
	WriteMutex *sync.Mutex
	Cache      *sync.Map
	PorterDB   *PorterDB
}

var PortSet Ports

func RandomPort(protocol string, connectionID string) uint16 {
	PortSet.WriteMutex.Lock()
	timeout := Parameters.PortTimeout
	if Parameters.PortTimeout <= 0 {
		timeout = DEFAULT_PORT_CACHE_TIME
	}

	key := fmt.Sprintf("%s-%s", protocol, connectionID)
	if port, ok := PortSet.Cache.Load(key); ok {
		if time.Now().After(port.(*UsingPort).Timestamp.Add(timeout * time.Second)) {
			PortSet.Cache.Delete(key)
		} else {
			port.(*UsingPort).Timestamp = time.Now()
			PortSet.WriteMutex.Unlock()
			return port.(*UsingPort).Port
		}
	}

	start := rand.Intn(PortSet.ranges)
	for {
		port := uint16(start + PortSet.begin)
		if _, ok := PortSet.usingPorts.Load(port); !ok {
			switch protocol {
			case "udp":
				PortSet.usingPorts.Store(port, protocols{udp: true})
			case "kcp":
				PortSet.usingPorts.Store(port, protocols{kcp: true})
			case "tcp":
				conn, err := net.Dial(protocol, fmt.Sprintf("%s:%d", GetLocalIP(), port))
				if conn != nil && err == nil {
					log.Error("What a superise, another goroutine is using the same port:", port)
					conn.Close()
					continue
				}
				PortSet.usingPorts.Store(port, protocols{tcp: true})
			case "quic":
				PortSet.usingPorts.Store(port, protocols{quic: true})
			default:
				PortSet.WriteMutex.Unlock()
				log.Error("not support ", protocol, ", please use tcp/kcp/udp/quic.")
				return 0
			}
			PortSet.Cache.Store(key, &UsingPort{Timestamp: time.Now(), ConnectionID: connectionID, Port: port, Protocol: protocol})
			PortSet.WriteMutex.Unlock()
			return port
		} else {
			start += 1
		}
	}
}

func InitPorts() {
	PortSet = Ports{
		begin:      Parameters.RandomPortBegin,
		ranges:     Parameters.RandomPortRange,
		usingPorts: new(sync.Map),
		WriteMutex: new(sync.Mutex),
		Cache:      new(sync.Map),
		PorterDB:   OpenPorterDB(Parameters.PorterDBPath),
	}
}
