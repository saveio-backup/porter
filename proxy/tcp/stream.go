/**
 * Description:
 * Author: Yihen.Liu
 * Create: 2019-04-26
 */
package kcp

import (
	"encoding/binary"
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"github.com/saveio/porter/common"
	"github.com/saveio/porter/internal/protobuf"
	"github.com/saveio/porter/types/opcode"
	"github.com/saveio/themis/common/log"
	"sync/atomic"
	"io"
)

const defaultRecvBufferSize = 4 * 1024 * 1024

func receiveTcpRawMessage(state *ConnState) ([]byte, error) {
	var err error
	var size uint32
	// Read until all header bytes have been read.
	sizeBuf := make([]byte, 4)
	bytesRead, totalBytesRead := 0, 0

	for totalBytesRead < 4 && err == nil {
		//bytesRead, err = state.conn.Read(sizeBuf[totalBytesRead:])
		bytesRead, err = io.ReadFull(state.conn, sizeBuf[totalBytesRead:])
		totalBytesRead += bytesRead
	}
	size = binary.BigEndian.Uint32(sizeBuf)
	buffer := make([]byte, size)

	bytesRead, totalBytesRead = 0, 0

	for totalBytesRead < int(size) && err == nil {
		//bytesRead, err = state.conn.Read(buffer[totalBytesRead:])
		bytesRead, err = io.ReadFull(state.conn, buffer[totalBytesRead:])
		totalBytesRead += bytesRead
	}

	return append(sizeBuf, buffer...), nil
}

func receiveMessage(state *ConnState) (*protobuf.Message, error) {
	var err error
	var size uint32
	// Read until all header bytes have been read.
	buffer := make([]byte, 4)
	bytesRead, totalBytesRead := 0, 0

	for totalBytesRead < 4 && err == nil {
		bytesRead, err = state.conn.Read(buffer[totalBytesRead:])
		//bytesRead, err = io.ReadFull(state.conn, buffer[totalBytesRead:])
		totalBytesRead += bytesRead
	}
	size = binary.BigEndian.Uint32(buffer)

	// Read until all message bytes have been read.
	buffer = make([]byte, size)

	bytesRead, totalBytesRead = 0, 0

	for totalBytesRead < int(size) && err == nil {
		bytesRead, err = state.conn.Read(buffer[totalBytesRead:])
		//bytesRead, err = io.ReadFull(state.conn, buffer[totalBytesRead:])
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

func prepareMessage(message proto.Message, state *ConnState) *protobuf.Message {
	bytes, err := proto.Marshal(message)
	if err != nil {
		log.Error("in prepareMessage, (first) Marshal Message, ERROR:", err.Error())
	}

	opcode, err := opcode.GetOpcode(message)
	if err != nil {
		return nil
	}

	return &protobuf.Message{
		Opcode:       uint32(opcode),
		Message:      bytes,
		Sender:       &protobuf.ID{Address: common.GetPublicHost("tcp")},
		MessageNonce: atomic.AddUint64(&state.messageNonce, 1),
	}
}

func sendMessage(state *ConnState, message proto.Message) error {
	bytes, err := proto.Marshal(prepareMessage(message, state))
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

	state.writerMutex.Lock()
	defer state.writerMutex.Unlock()

	//bw, isBuffered := w.(*bufio.Writer)
	if (state.writer.Buffered() > 0) && (state.writer.Available() < totalSize) {
		if err := state.writer.Flush(); err != nil {
			return err
		}
	}

	for totalBytesWritten < len(buffer) && err == nil {
		bytesWritten, err = state.writer.Write(buffer[totalBytesWritten:])
		if err != nil {
			log.Errorf("stream(common): failed to write entire buffer, err: %+v", err)
		}
		totalBytesWritten += bytesWritten
	}

	if err != nil {
		return errors.Wrap(err, "stream: failed to write to socket")
	}

	return nil
}

func transferTcpRawMessage(message []byte, state *ConnState) error {
	totalSize := len(message)

	// Write until all bytes have been written.
	bytesWritten, totalBytesWritten := 0, 0

	state.writerMutex.Lock()
	defer state.writerMutex.Unlock()

	if (state.writer.Buffered() > 0) && (state.writer.Available() < totalSize) {
		if err := state.writer.Flush(); err != nil {
			return err
		}
	}
	var err error
	for totalBytesWritten < len(message) && err == nil {
		bytesWritten, err = state.writer.Write(message[totalBytesWritten:])
		if err != nil {
			log.Errorf("stream(raw): failed to write entire buffer, err: %+v", err)
		}
		totalBytesWritten += bytesWritten
	}

	if err != nil {
		return errors.Wrap(err, "stream: failed to write to socket")
	}

	return nil
}
