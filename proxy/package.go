/**
 * Description:
 * Author: Yihen.Liu
 * Create: 2019-04-26 
*/
package proxy

import (
	"github.com/oniio/oniProxy/internal/protobuf"
	"encoding/binary"
	"github.com/golang/protobuf/proto"
	"github.com/oniio/oniChain/common/log"
	"net"
	"fmt"
	"github.com/oniio/oniProxy/types/opcode"
)

func receiveUDPRawMessage(conn *net.UDPConn) []byte {
	buffer := make([]byte, MAX_PACKAGE_SIZE)
	length, remoteAddr, err :=conn.ReadFromUDP(buffer)
	if remoteAddr == nil && length == 0 || err !=nil {
		return  nil
	}else {
		return buffer[:length]
	}
}

func receiveUDPMessage(conn *net.UDPConn) (*protobuf.Message) {
	buffer := make([]byte, MAX_PACKAGE_SIZE)
	length, remoteAddr, err :=conn.ReadFromUDP(buffer)
	if remoteAddr == nil && length == 0 || err !=nil {
		return  nil
	}

	size := binary.BigEndian.Uint16(buffer[0:2])
	msg := new(protobuf.Message)
	err = proto.Unmarshal(buffer[2:2+size], msg)
	if err!=nil{
		log.Errorf("receive udp message error:", err.Error())
		return nil
	}
	return msg
}

func prepareMessage(message proto.Message) ([]byte) {
	bytes, err := proto.Marshal(message)
	if err != nil {
		log.Error("in prepareMessage, (first) Marshal Message, ERROR:", err.Error())
	}
	msg := &protobuf.Message{
		Opcode: 	uint32(opcode.ProxyCode),
		IsProxy:	true,
		Message: 	bytes,
		Sender: 	&protobuf.ID{Address:"udp://127.0.0.1:6008",},
	}
	raw, err :=proto.Marshal(msg)
	if err != nil {
		log.Error("in prepareMessage, (second) Marshal Message, ERROR:", err.Error())
	}
	buffer := make([]byte, 2)
	binary.BigEndian.PutUint16(buffer, uint16(len(raw)))
	buffer = append(buffer, raw...)

	return buffer
}

func sendUDPMessage(message proto.Message,  udpConn *net.UDPConn, remoteAddr string) {
	addrInfo, err:=ParseAddress(remoteAddr)
	resolved, err := net.ResolveUDPAddr("udp", addrInfo.toString())
	if err != nil {
		log.Error("in serverAccept, resolve:",err.Error())
	}

	buffer := prepareMessage(message)
	_, err=udpConn.WriteToUDP(buffer, resolved)
	if err!=nil{
		fmt.Println("err:", err.Error())
	}
}

func transferUDPRawMessage(message []byte,  udpConn *net.UDPConn, remoteAddr string) {
	addrInfo, err:=ParseAddress(remoteAddr)
	resolved, err := net.ResolveUDPAddr("udp", addrInfo.toString())
	if err != nil {
		log.Error("in serverAccept, resolve:",err.Error())
	}
	_, err=udpConn.WriteToUDP(message, resolved)
	if err!=nil{
		fmt.Println("err:", err.Error())
	}
}
