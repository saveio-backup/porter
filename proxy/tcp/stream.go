/**
 * Description:
 * Author: Yihen.Liu
 * Create: 2019-04-26
 */
package tcp

import (
	"encoding/binary"
	"fmt"
	"io"
	"runtime/debug"
	"sync/atomic"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"github.com/saveio/porter/common"
	"github.com/saveio/porter/internal/protobuf"
	"github.com/saveio/porter/types/opcode"
	"github.com/saveio/themis/common/log"
)

const defaultRecvBufferSize = 4 * 1024 * 1024

func receiveTcpRawMessage(state *ConnState, sendTo string) ([]byte, error, string, uint32, uint64) {
	var err error
	var size uint32
	// Read until all header bytes have been read.
	sizeBuf := make([]byte, 4)
	bytesRead, totalBytesRead := 0, 0

	for totalBytesRead < 4 && err == nil {
		bytesRead, err = io.ReadFull(state.conn, sizeBuf[totalBytesRead:])
		if err != nil || bytesRead == 0 {
			log.Error("tcp receive raw message head err:", err.Error(), "has read buffer message:", sizeBuf, "buffer.len:", bytesRead)
			return nil, err, "", 0, 0
		}
		totalBytesRead += bytesRead
	}

	if binary.BigEndian.Uint32(sizeBuf) != common.Parameters.NetworkID {
		return nil, errors.New("networkID is not match the message info which is contained in msg 4 bytes ahead when recv Raw Message"), "", 0, 0
	}

	sizeBuf = make([]byte, 4)
	bytesRead, totalBytesRead = 0, 0

	for totalBytesRead < 4 && err == nil {
		bytesRead, err = io.ReadFull(state.conn, sizeBuf[totalBytesRead:])
		if err != nil || bytesRead == 0 {
			log.Error("tcp receive raw message head err:", err.Error(), "has read buffer message:", sizeBuf, "buffer.len:", bytesRead)
			return nil, err, "", 0, 0
		}
		totalBytesRead += bytesRead
	}

	size = binary.BigEndian.Uint32(sizeBuf)
	if size == 0 {
		return nil, errors.New("message body size is zero in head expression when recvTcpRawMsg"), "", 0, 0
	}
	buffer := make([]byte, size)

	bytesRead, totalBytesRead = 0, 0

	for totalBytesRead < int(size) && err == nil {
		bytesRead, err = io.ReadFull(state.conn, buffer[totalBytesRead:])
		if err != nil || bytesRead == 0 {
			log.Error("tcp receive raw message body err:", err.Error(), "has read buffer message:", buffer[:totalBytesRead+bytesRead], "buffer.len:", bytesRead, "total message body size:", size)
			return nil, err, "", 0, 0
		}
		totalBytesRead += bytesRead
	}
	if err != nil {
		return nil, err, "", 0, 0
	}

	// Deserialize message.
	msg := new(protobuf.Message)
	err = proto.Unmarshal(buffer, msg)
	if err != nil {
		return nil, errors.New("failed to unmarshal message in receiveTcpRawMessage"), "", 0, 0
	}
	log.Info("in receiveTcpRawMessage recv a message will be transfered, sender from:", msg.Sender.Address, ",send to:", sendTo, ",msg.opcode:", msg.Opcode, ",msg.Nonce:", msg.GetMessageNonce(), ",networkID:", msg.NetID)
	return append(sizeBuf, buffer...), nil, msg.Sender.Address, msg.Opcode, msg.GetMessageNonce()
}

func receiveMessage(state *ConnState) (*protobuf.Message, error) {
	var err error
	var size uint32
	// Read until all header bytes have been read.
	buffer := make([]byte, 4)
	bytesRead, totalBytesRead := 0, 0
	for totalBytesRead < 4 && err == nil {
		bytesRead, err = state.conn.Read(buffer[totalBytesRead:])
		if err != nil || bytesRead == 0 {
			log.Error("tcp receive networkID message head err:", err.Error(), ",has read buffer message:", buffer, ",buffer.len:", bytesRead)
			return nil, err
		}
		totalBytesRead += bytesRead
	}

	if binary.BigEndian.Uint32(buffer) != common.Parameters.NetworkID {
		return nil, errors.Errorf("networkID is not match the message info which is contained in msg 4 bytes ahead when recvMessage, "+
			"recv.NetID:%d, setting.NetID:%d", binary.BigEndian.Uint32(buffer), common.Parameters.NetworkID)
	}

	buffer = make([]byte, 4)
	bytesRead, totalBytesRead = 0, 0
	for totalBytesRead < 4 && err == nil {
		bytesRead, err = state.conn.Read(buffer[totalBytesRead:])
		if err != nil || bytesRead == 0 {
			log.Error("tcp receive message head err:", err.Error(), ",has read buffer message:", buffer, ",buffer.len:", bytesRead)
			return nil, err
		}
		totalBytesRead += bytesRead
	}
	size = binary.BigEndian.Uint32(buffer)
	if size == 0 {
		return nil, errors.New("message body size is zero when recvTcpMsg")
	}
	// Read until all message bytes have been read.
	buffer = make([]byte, size)

	bytesRead, totalBytesRead = 0, 0

	for totalBytesRead < int(size) && err == nil {
		bytesRead, err = state.conn.Read(buffer[totalBytesRead:])
		if err != nil || bytesRead == 0 {
			log.Error("tcp receive message body err:", err.Error(), ",has read buffer message:",
				buffer[:totalBytesRead+bytesRead], ",buffer.len:", bytesRead, "total message body size:", size)
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
		Sender:       &protobuf.ID{Address: common.GetPublicHost("tcp"), NetKey: []byte("PORTER_TCP_NETKEY")},
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
		log.Error("stack info:", fmt.Sprintf("%s", debug.Stack()))
		return errors.New("tcp sendMessage,len(message) is empty")
	}

	// Serialize size.
	buffer := make([]byte, 8)
	binary.BigEndian.PutUint32(buffer, common.Parameters.NetworkID)
	binary.BigEndian.PutUint32(buffer[4:], uint32(len(bytes)))

	buffer = append(buffer, bytes...)

	// Write until all bytes have been written.
	bytesWritten, totalBytesWritten := 0, 0

	state.writerMutex.Lock()
	defer state.writerMutex.Unlock()

	//bw, isBuffered := w.(*bufio.Writer)

	for totalBytesWritten < len(buffer) && err == nil {
		bytesWritten, err = state.writer.Write(buffer[totalBytesWritten:])
		if err != nil {
			log.Errorf("tcp stream(common): failed to write entire buffer, err: %+v", err)
			return err
		}
		totalBytesWritten += bytesWritten
	}

	if err != nil {
		return errors.Wrap(err, "tcp stream: failed to write to socket")
	}

	if err := state.writer.Flush(); err != nil {
		log.Errorf("tcp stream(common): failed to flush buffer, err: %+v", err)
		return err
	}

	return nil
}

func transferTcpRawMessage(message []byte, state *ConnState) error {
	if len(message) == 0 {
		return errors.New("in transferTCPRawMessage, will send empty message")
	}
	// Write until all bytes have been written.
	bytesWritten, totalBytesWritten := 0, 0

	buffer := make([]byte, 4)
	binary.BigEndian.PutUint32(buffer, common.Parameters.NetworkID)

	buffer = append(buffer, message...)

	state.writerMutex.Lock()
	defer state.writerMutex.Unlock()

	var err error
	for totalBytesWritten < len(buffer) && err == nil {
		bytesWritten, err = state.writer.Write(buffer[totalBytesWritten:])
		if err != nil {
			log.Errorf("tcp stream(raw): failed to write entire buffer, err: %+v", err)
			return err
		}
		totalBytesWritten += bytesWritten
	}

	if err != nil {
		return errors.Wrap(err, "(tcp) stream: failed to write to socket")
	}

	if err := state.writer.Flush(); err != nil {
		log.Errorf("tcp stream(raw): failed to flush entire buffer, err: %+v", err)
		return err
	}

	return nil
}
