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

func  receiveKCPRawMessage(conn net.Conn) ([]byte, error) {
	var err error
	var size uint32
	// Read until all header bytes have been read.
	sizeBuf := make([]byte, 4)
	bytesRead, totalBytesRead := 0, 0

	for totalBytesRead < 4 && err == nil {
		bytesRead, err = conn.Read(sizeBuf[totalBytesRead:])
		totalBytesRead += bytesRead
	}
	size = binary.BigEndian.Uint32(sizeBuf)
	buffer := make([]byte, size)

	bytesRead, totalBytesRead = 0, 0

	for totalBytesRead < int(size) && err == nil {
		bytesRead, err = conn.Read(buffer[totalBytesRead:])

		//bytesRead, err = conn.Read(buffer[totalBytesRead:])
		totalBytesRead += bytesRead
	}

	return append(sizeBuf, buffer...), nil
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
	// Check if any of the message headers are invalid or null.
	if msg.Opcode == 0 || msg.Sender == nil || msg.Sender.NetKey == nil || len(msg.Sender.Address) == 0 {
		return nil, errors.New("received an invalid message (either no opcode, no sender, no net key, or no signature) from a peer")
	}
	return msg, nil
}

func prepareMessage(message proto.Message) (*protobuf.Message) {
	bytes, err := proto.Marshal(message)
	if err != nil {
		log.Error("in prepareMessage, (first) Marshal Message, ERROR:", err.Error())
	}
	return  &protobuf.Message{
		Opcode: 	uint32(opcode.ProxyResponseCode),
		Message: 	bytes,
		Sender: 	&protobuf.ID{Address:fmt.Sprintf("kcp://%s:6008", common.GetLocalIP()),},
	}
}

func sendMessage( conn net.Conn, message proto.Message) error {
	w:=bufio.NewWriterSize(conn, defaultRecvBufferSize)
	bytes, err := proto.Marshal(prepareMessage(message))
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
		err= w.Flush()
		if err != nil {
			log.Errorf("flush err: %+v", err)
		}
	}

	//writerMutex.Unlock()
	if err != nil {
		return errors.Wrap(err, "stream: failed to write to socket")
	}

	return nil
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
	err= w.Flush()
	if err != nil {
		log.Errorf("flush err: %+v", err)
	}
	//writerMutex.Unlock()

	if err != nil {
		return errors.Wrap(err, "stream: failed to write to socket")
	}

	return nil
}