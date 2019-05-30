/**
 * Description:
 * Author: Yihen.Liu
 * Create: 2019-05-09
 */
package common

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
)

// AddressInfo represents a network URL.
type addressInfo struct {
	Protocol string
	Host     string
	Port     uint16
}

func (addr addressInfo) ToString() string {
	return fmt.Sprintf("%s:%d", addr.Host, addr.Port)
}

// ParseAddress derives a network scheme, host and port of a destinations
// information. Errors should the provided destination address be malformed.
//protocol://ip:port
func ParseAddress(address string) (*addressInfo, error) {
	urlInfo, err := url.Parse(address)
	if err != nil {
		return nil, err
	}

	host, rawPort, err := net.SplitHostPort(urlInfo.Host)
	if err != nil {
		return nil, err
	}

	port, err := strconv.ParseUint(rawPort, 10, 16)
	if err != nil {
		return nil, err
	}

	return &addressInfo{
		Protocol: urlInfo.Scheme,
		Host:     host,
		Port:     uint16(port),
	}, nil
}
