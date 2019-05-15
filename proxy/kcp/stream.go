/**
 * Description:
 * Author: Yihen.Liu
 * Create: 2019-04-26 
*/
package kcp

import (
	"github.com/saveio/porter/internal/protobuf"
	"encoding/binary"
	"github.com/golang/protobuf/proto"
	"github.com/saveio/themis/common/log"
	"net"
	"fmt"
	"github.com/saveio/porter/types/opcode"
	"github.com/saveio/porter/common"
	"github.com/pkg/errors"
	"bufio"
)

const defaultRecvBufferSize    = 4 * 1024 * 1024

func receiveUDPRawMessage(conn *net.UDPConn) ([]byte, error) {
	buffer := make([]byte, MAX_PACKAGE_SIZE)
	length, remoteAddr, err :=conn.ReadFromUDP(buffer)
	if remoteAddr == nil && length == 0 || err !=nil {
		return  nil, err
	}else {
		return buffer[:length], nil
	}
}

func  receiveKCPRawMessage(conn net.Conn) ([]byte, error) {
	var err error
	var size uint32
	// Read until all header bytes have been read.
	buffer := make([]byte, 4)
	bytesRead, totalBytesRead := 0, 0

	for totalBytesRead < 4 && err == nil {
		bytesRead, err = conn.Read(buffer[totalBytesRead:])
		totalBytesRead += bytesRead
	}
	size = binary.BigEndian.Uint32(buffer)

	// Read until all message bytes have been read.
	buffer = make([]byte, size)

	bytesRead, totalBytesRead = 0, 0

	for totalBytesRead < int(size) && err == nil {
		bytesRead, err = conn.Read(buffer[totalBytesRead:])

		//bytesRead, err = conn.Read(buffer[totalBytesRead:])
		totalBytesRead += bytesRead
	}

	return buffer, nil
}

func  receiveMessage(conn net.Conn) (*protobuf.Message, error) {
	var err error
	var size uint32
	// Read until all header bytes have been read.
	buffer := make([]byte, 4)
	bytesRead, totalBytesRead := 0, 0

	for totalBytesRead < 4 && err == nil {
		bytesRead, err = conn.Read(buffer[totalBytesRead:])
		totalBytesRead += bytesRead
	}
	size = binary.BigEndian.Uint32(buffer)

	// Read until all message bytes have been read.
	buffer = make([]byte, size)

	bytesRead, totalBytesRead = 0, 0

	for totalBytesRead < int(size) && err == nil {
		bytesRead, err = conn.Read(buffer[totalBytesRead:])

		//bytesRead, err = conn.Read(buffer[totalBytesRead:])
		totalBytesRead += bytesRead
	}

	// Deserialize message.
	msg := new(protobuf.Message)
	err = proto.Unmarshal(buffer, msg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal message")
	}
	fmt.Println("receive: ", buffer)
	// Check if any of the message headers are invalid or null.
	if msg.Opcode == 0 || msg.Sender == nil || msg.Sender.NetKey == nil || len(msg.Sender.Address) == 0 {
		return nil, errors.New("received an invalid message (either no opcode, no sender, no net key, or no signature) from a peer")
	}
	return msg, nil
}

func receiveUDPProxyMessage(conn *net.UDPConn) (*protobuf.Message, string) {
	buffer := make([]byte, MAX_PACKAGE_SIZE)
	length, remoteAddr, err :=conn.ReadFromUDP(buffer)
	if remoteAddr == nil && length == 0 || err !=nil {
		return  nil, ""
	}

	size := binary.BigEndian.Uint16(buffer[0:2])
	msg := new(protobuf.Message)
	err = proto.Unmarshal(buffer[2:2+size], msg)
	if err!=nil{
		log.Errorf("receive udp message error:", err.Error())
		return nil, ""
	}
	return msg, fmt.Sprintf("udp://%s",remoteAddr)
}

func prepareMessage(message proto.Message) ([]byte) {
	bytes, err := proto.Marshal(message)
	if err != nil {
		log.Error("in prepareMessage, (first) Marshal Message, ERROR:", err.Error())
	}
	msg := &protobuf.Message{
		Opcode: 	uint32(opcode.ProxyResponseCode),
		Message: 	bytes,
		Sender: 	&protobuf.ID{Address:fmt.Sprintf("udp://%s:6008", common.GetLocalIP()),},
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

func sendMessage( conn net.Conn, message proto.Message) error {
	w:=bufio.NewWriterSize(conn, defaultRecvBufferSize)
	bytes, err := proto.Marshal(message)
	if err != nil {
		return errors.Wrap(err, "failed to marshal message")
	}

	// Serialize size.
	buffer := make([]byte, 4)
	binary.BigEndian.PutUint32(buffer, uint32(len(bytes)))

	buffer = append(buffer, bytes...)
	totalSize := len(buffer)

	// Write until all bytes have been written.
	bytesWritten, totalBytesWritten := 0, 0

	//writerMutex.Lock()

	//bw, isBuffered := w.(*bufio.Writer)
	if  (w.Buffered() > 0) && (w.Available() < totalSize) {
		if err := w.Flush(); err != nil {
			return err
		}
	}

	for totalBytesWritten < len(buffer) && err == nil {
		bytesWritten, err = w.Write(buffer[totalBytesWritten:])
		if err != nil {
			log.Errorf("stream: failed to write entire buffer, err: %+v", err)
		}
		totalBytesWritten += bytesWritten
	}

/*	select {
	case <-n.kill:
		if err := bw.Flush(); err != nil {
			return err
		}
	default:
	}*/

	//writerMutex.Unlock()
	if err != nil {
		return errors.Wrap(err, "stream: failed to write to socket")
	}

	return nil
}

func sendUDPMessage(message proto.Message,  udpConn *net.UDPConn, remoteAddr string) {
	addrInfo, err:=common.ParseAddress(remoteAddr)
	resolved, err := net.ResolveUDPAddr("udp", addrInfo.ToString())
	if err != nil {
		log.Error("in serverAccept, resolve:",err.Error())
	}

	buffer := prepareMessage(message)
	_, err=udpConn.WriteToUDP(buffer, resolved)
	if err!=nil{
		log.Error("err:", err.Error())
	}
}

func transferKCPRawMessage(message []byte,  conn net.Conn) error {
	w:=bufio.NewWriterSize(conn, defaultRecvBufferSize)
	totalSize := len(message)

	// Write until all bytes have been written.
	bytesWritten, totalBytesWritten := 0, 0

	//writerMutex.Lock()

	//bw, isBuffered := w.(*bufio.Writer)
	if  (w.Buffered() > 0) && (w.Available() < totalSize) {
		if err := w.Flush(); err != nil {
			return err
		}
	}
	var err error
	for totalBytesWritten < len(message) && err == nil {
		bytesWritten, err = w.Write(message[totalBytesWritten:])
		if err != nil {
			log.Errorf("stream: failed to write entire buffer, err: %+v", err)
		}
		totalBytesWritten += bytesWritten
	}

	/*	select {
		case <-n.kill:
			if err := bw.Flush(); err != nil {
				return err
			}
		default:
		}*/

	//writerMutex.Unlock()

	if err != nil {
		return errors.Wrap(err, "stream: failed to write to socket")
	}

	return nil
}

func transferUDPRawMessage(message []byte,  udpConn *net.UDPConn, remoteAddr string) {
	addrInfo, err:=common.ParseAddress(remoteAddr)
	resolved, err := net.ResolveUDPAddr("udp", addrInfo.ToString())
	if err != nil {
		log.Error("in serverAccept, resolve:",err.Error())
	}
	_, err=udpConn.WriteToUDP(message, resolved)
	if err!=nil{
		log.Error("err:", err.Error())
	}
}
