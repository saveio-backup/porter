/**
 * Description:
 * Author: Yihen.Liu
 * Create: 2019-04-26
 */
package quic

import (
	"encoding/binary"
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"github.com/saveio/porter/common"
	"github.com/saveio/porter/internal/protobuf"
	"github.com/saveio/porter/types/opcode"
	"github.com/saveio/themis/common/log"
	"io"
	"runtime/debug"
	"sync/atomic"
)

const defaultRecvBufferSize = 4 * 1024 * 1024

func receiveQuicRawMessage(state *ConnState) ([]byte, error) {
	var err error
	var size uint32
	// Read until all header bytes have been read.
	sizeBuf := make([]byte, 4)
	bytesRead, totalBytesRead := 0, 0

	for totalBytesRead < 4 && err == nil {
		bytesRead, err = io.ReadFull(state.conn, sizeBuf[totalBytesRead:])
		if err != nil || bytesRead == 0 {
			log.Error("quic receive message head err:", err.Error(), "has read buffer message:", sizeBuf, "buffer.len:", bytesRead)
			return nil, err
		}
		totalBytesRead += bytesRead
	}
	if err != nil {
		return nil, err
	}
	size = binary.BigEndian.Uint32(sizeBuf)
	buffer := make([]byte, size)

	bytesRead, totalBytesRead = 0, 0

	for totalBytesRead < int(size) && err == nil {
		//bytesRead, err = state.conn.Read(buffer[totalBytesRead:])
		bytesRead, err = io.ReadFull(state.conn, buffer[totalBytesRead:])
		if err != nil || bytesRead == 0 {
			log.Error("quic receive message head err:", err.Error(), "has read buffer message:", sizeBuf, "buffer.len:", bytesRead)
			return nil, err
		}
		totalBytesRead += bytesRead
	}
	totalBytesRead += bytesRead
	return append(sizeBuf, buffer...), nil
}

func receiveMessage(state *ConnState) (*protobuf.Message, error) {
	var err error
	var size uint32
	// Read until all header bytes have been read.
	buffer := make([]byte, 4)
	bytesRead, totalBytesRead := 0, 0

	for totalBytesRead < 4 && err == nil {
		bytesRead, err = io.ReadFull(state.conn, buffer[totalBytesRead:])
		if err != nil || bytesRead == 0 {
			log.Error("quic receive message head err:", err.Error(), "has read buffer message:", buffer, "buffer.len:", bytesRead)
			return nil, err
		}
		totalBytesRead += bytesRead
	}
	size = binary.BigEndian.Uint32(buffer)

	// Read until all message bytes have been read.
	buffer = make([]byte, size)

	bytesRead, totalBytesRead = 0, 0

	for totalBytesRead < int(size) && err == nil {
		bytesRead, err = io.ReadFull(state.conn, buffer[totalBytesRead:])
		if err != nil || bytesRead == 0 {
			log.Error("quic receive message body err:", err.Error(), "has read buffer message:", buffer, "buffer.len:", bytesRead)
			return nil, err
		}
		totalBytesRead += bytesRead
	}

	// Deserialize message.
	msg := new(protobuf.Message)
	err = proto.Unmarshal(buffer, msg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal message")
	}
	// Check if any of the message headers are invalid or null.
	if msg.Opcode == 0 || msg.Sender == nil || msg.Sender.NetKey == nil || len(msg.Sender.Address) == 0 || msg.NetID != common.Parameters.NetworkID {
		return nil, errors.New("received an invalid message (either no opcode, no sender, no net key, no signature, or networkID invalid) from a peer")
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
		Sender:       &protobuf.ID{Address: common.GetPublicHost("quic"), NetKey: []byte("PORTER_QUIC_NETKEY")},
		MessageNonce: atomic.AddUint64(&state.messageNonce, 1),
		NetID:        common.Parameters.NetworkID,
	}
}

func sendMessage(state *ConnState, message proto.Message) error {
	bytes, err := proto.Marshal(prepareMessage(message, state))
	if err != nil {
		return errors.Wrap(err, "failed to marshal message")
	}
	if len(bytes) == 0 {
		log.Error("(quic)stack info:", fmt.Sprintf("%s", debug.Stack()))
		return errors.New("quic sendMessage,len(message) is empty")
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

	if (state.writer.Buffered() > 0) && (state.writer.Available() < totalSize) {
		if err := state.writer.Flush(); err != nil {
			log.Errorf("quic stream(common): flush err in sendMessage, err: %+v", err)
			return err
		}
	}

	for totalBytesWritten < len(buffer) && err == nil {
		bytesWritten, err = state.writer.Write(buffer[totalBytesWritten:])
		if err != nil {
			log.Errorf("quic stream(common): failed to write entire buffer, err: %+v", err)
			return err
		}
		totalBytesWritten += bytesWritten
	}

	if err != nil {
		return errors.Wrap(err, "quic stream: failed to write to socket")
	}

	return nil
}

func transferQuicRawMessage(message []byte, state *ConnState) error {
	totalSize := len(message)
	if totalSize == 0 {
		return errors.New("in transferQuicRawMessage, will send empty message")
	}
	// Write until all bytes have been written.
	bytesWritten, totalBytesWritten := 0, 0

	state.writerMutex.Lock()
	defer state.writerMutex.Unlock()

	if (state.writer.Buffered() > 0) && (state.writer.Available() < totalSize) {
		if err := state.writer.Flush(); err != nil {
			log.Errorf("quic stream(raw): transfer quic raw message err: %+v", err)
			return err
		}
	}
	var err error
	for totalBytesWritten < len(message) && err == nil {
		bytesWritten, err = state.writer.Write(message[totalBytesWritten:])
		if err != nil {
			log.Errorf("quic stream(raw): failed to write entire buffer, err: %+v", err)
			return err
		}
		totalBytesWritten += bytesWritten
	}

	if err != nil {
		return errors.Wrap(err, "quic stream: failed to write to socket")
	}

	return nil
}
