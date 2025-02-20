/**
 * Description:
 * Author: Yihen.Liu
 * Create: 2019-04-26
 */
package udp

import (
	"encoding/binary"
	"fmt"
	"net"

	"github.com/golang/protobuf/proto"
	"github.com/saveio/porter/common"
	"github.com/saveio/porter/internal/protobuf"
	"github.com/saveio/porter/types/opcode"
	"github.com/saveio/themis/common/log"
)

func receiveUDPRawMessage(conn *net.UDPConn) ([]byte, error) {
	buffer := make([]byte, MAX_PACKAGE_SIZE)
	length, remoteAddr, err := conn.ReadFromUDP(buffer)
	if remoteAddr == nil && length == 0 || err != nil {
		return nil, err
	} else {
		return buffer[:length], nil
	}
}

func receiveUDPProxyMessage(conn *net.UDPConn) (*protobuf.Message, string) {
	buffer := make([]byte, MAX_PACKAGE_SIZE)
	length, remoteAddr, err := conn.ReadFromUDP(buffer)
	if remoteAddr == nil && length == 0 || err != nil {
		return nil, ""
	}

	size := binary.BigEndian.Uint16(buffer[0:2])
	msg := new(protobuf.Message)
	err = proto.Unmarshal(buffer[2:2+size], msg)
	if err != nil {
		log.Errorf("receive udp message error:", err.Error())
		return nil, ""
	}
	return msg, fmt.Sprintf("udp://%s", remoteAddr)
}

func prepareMessage(message proto.Message) []byte {
	bytes, err := proto.Marshal(message)
	if err != nil {
		log.Error("in prepareMessage, (first) Marshal Message, ERROR:", err.Error())
	}
	codeNum, _ := opcode.GetOpcode(message)
	msg := &protobuf.Message{
		Opcode:  uint32(codeNum),
		Message: bytes,
		Sender:  &protobuf.ID{Address: common.GetPublicHost("udp"), NetKey: []byte("PORTER_UDP_NETKEY")},
		NetID:   common.Parameters.NetworkID,
	}
	raw, err := proto.Marshal(msg)
	if err != nil {
		log.Error("in prepareMessage, (second) Marshal Message, ERROR:", err.Error())
	}
	buffer := make([]byte, 2)
	binary.BigEndian.PutUint16(buffer, uint16(len(raw)))
	buffer = append(buffer, raw...)

	return buffer
}

func sendUDPMessage(message proto.Message, udpConn *net.UDPConn, remoteAddr string) error {
	addrInfo, err := common.ParseAddress(remoteAddr)
	resolved, err := net.ResolveUDPAddr("udp", addrInfo.ToString())
	if err != nil {
		log.Error("in serverAccept, resolve:", err.Error())
		return err
	}

	buffer := prepareMessage(message)
	_, err = udpConn.WriteToUDP(buffer, resolved)
	if err != nil {
		log.Error("err:", err.Error())
		return err
	}
	return nil
}

func transferUDPRawMessage(message []byte, udpConn *net.UDPConn, remoteAddr string) {
	addrInfo, err := common.ParseAddress(remoteAddr)
	resolved, err := net.ResolveUDPAddr("udp", addrInfo.ToString())
	if err != nil {
		log.Error("in serverAccept, resolve:", err.Error())
	}
	_, err = udpConn.WriteToUDP(message, resolved)
	if err != nil {
		log.Error("err:", err.Error())
	}
}
