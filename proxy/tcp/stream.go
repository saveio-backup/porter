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

func (p *TCPProxyServer) receiveTcpRawMessage(state *ConnState, sendTo string) ([]byte, error) {
	var err error
	var size uint32
	// Read until all header bytes have been read.
	sizeBuf := make([]byte, 4)
	bytesRead, totalBytesRead := 0, 0

	for totalBytesRead < 4 && err == nil {
		bytesRead, err = io.ReadFull(state.conn, sizeBuf[totalBytesRead:])
		if err != nil || bytesRead == 0 {
			log.Error("tcp receive raw message head err:", err.Error(), "has read buffer message:", sizeBuf, "buffer.len:", bytesRead)
			return nil, err
		}
		totalBytesRead += bytesRead
	}
	//start := time.Now()
	if binary.BigEndian.Uint32(sizeBuf) != common.Parameters.NetworkID {
		return nil, errors.New("networkID is not match the message info which is contained in msg 4 bytes ahead when recv Raw Message")
	}

	algoBuf := make([]byte, 2)
	bytesRead, totalBytesRead = 0, 0

	for totalBytesRead < 2 && err == nil {
		bytesRead, err = io.ReadFull(state.conn, algoBuf[totalBytesRead:])
		totalBytesRead += bytesRead
	}

	if err != nil {
		return nil, errors.New("read compress algo information err")
	}

	compressInfo := binary.BigEndian.Uint16(algoBuf)
	algo, compEnable := common.GetCompressInfo(compressInfo)

	sizeBuf = make([]byte, 4)
	bytesRead, totalBytesRead = 0, 0

	for totalBytesRead < 4 && err == nil {
		bytesRead, err = io.ReadFull(state.conn, sizeBuf[totalBytesRead:])
		if err != nil || bytesRead == 0 {
			log.Error("tcp receive raw message head err:", err.Error(), "has read buffer message:", sizeBuf, "buffer.len:", bytesRead)
			return nil, err
		}
		totalBytesRead += bytesRead
	}

	size = binary.BigEndian.Uint32(sizeBuf)
	if size == 0 {
		return nil, errors.New("message body size is zero in head expression when recvTcpRawMsg")
	}
	buffer := make([]byte, size)

	bytesRead, totalBytesRead = 0, 0

	for totalBytesRead < int(size) && err == nil {
		bytesRead, err = io.ReadFull(state.conn, buffer[totalBytesRead:])
		if err != nil || bytesRead == 0 {
			log.Error("tcp receive raw message body err:", err.Error(), "has read buffer message:", buffer[:totalBytesRead+bytesRead], "buffer.len:", bytesRead, "total message body size:", size)
			return nil, err
		}
		totalBytesRead += bytesRead
	}
	//p.Metric.RecvRawTimeGauge.Update(int64(time.Since(start)))
	retBuf := append(algoBuf, append(sizeBuf, buffer...)...)

	go genDebugInfo(buffer, retBuf, compEnable, algo, sendTo)

	return retBuf, nil
}

func genDebugInfo(buffer, retBuf []byte, compEnable bool, algo common.AlgoType, sendTo string) {
	var err error
	if compEnable {
		buffer, err = common.Uncompress(buffer, algo)
		if err != nil {
			log.Error("uncompress buffer msg err, err:", err.Error(), ",algo type:", algo)
			return
		}
	}

	// Deserialize message.
	msg := new(protobuf.Message)
	err = proto.Unmarshal(buffer, msg)
	if err != nil {
		log.Errorf("failed to unmarshal message in receiveTcpRawMessage")
		return
	}
	log.Info("in receiveTcpRawMessage recv a message will be transfered, sender from:", msg.Sender.Address, ",send to:", sendTo,
		",msg.opcode:", msg.Opcode, ",msg.Nonce:", msg.GetMessageNonce(), ",networkID:", msg.NetID, ",msg.Len:", len(retBuf),
		",compress algo:", algo, ",compress enable:", compEnable)
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

	buffer = make([]byte, 2)
	bytesRead, totalBytesRead = 0, 0

	for totalBytesRead < 2 && err == nil {
		bytesRead, err = state.conn.Read(buffer[totalBytesRead:])
		totalBytesRead += bytesRead
	}

	if err != nil {
		return nil, errors.Errorf("tcp receive invalid message size bytes err:%s", err.Error())
	}

	compressInfo := binary.BigEndian.Uint16(buffer)
	algo, compEnable := common.GetCompressInfo(compressInfo)

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

	if compEnable {
		buffer, err = common.Uncompress(buffer, common.AlgoType(algo))
		if err != nil {
			log.Error("uncompress buffer msg err, err:", err.Error(), ",algo type:", algo)
			return nil, errors.Errorf("uncompress err:%s,algo:%d", err.Error(), algo)
		}
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

func replySyncMessage(state *ConnState, message proto.Message, nonce uint64) error {
	preMessage := prepareMessage(message, state)
	preMessage.ReplyFlag = true
	preMessage.RequestNonce = nonce
	bytes, err := proto.Marshal(preMessage)
	if err != nil {
		return errors.Wrap(err, "failed to marshal message in TCP sendMessage")
	}
	if len(bytes) == 0 {
		log.Error("stack info:", fmt.Sprintf("%s", debug.Stack()))
		return errors.New("tcp sendMessage,len(message) is empty")
	}

	var enable uint16
	if common.Parameters.Compression.Enable {
		enable = 1
	} else {
		enable = 0
	}
	// Serialize size.
	buffer := make([]byte, 10)
	binary.BigEndian.PutUint32(buffer, common.Parameters.NetworkID)
	binary.BigEndian.PutUint16(buffer[4:], uint16(enable<<8)|(uint16(common.Parameters.Compression.CompressAlgo)&0xFF))
	binary.BigEndian.PutUint32(buffer[6:], uint32(len(bytes)))

	buffer = append(buffer, bytes...)

	// Write until all bytes have been written.
	bytesWritten, totalBytesWritten := 0, 0

	state.writerMutex.Lock()
	defer state.writerMutex.Unlock()

	for totalBytesWritten < len(buffer) && err == nil {
		bytesWritten, err = state.writer.Write(buffer[totalBytesWritten:])
		if err != nil {
			log.Errorf("tcp stream(common): failed to write entire buffer, err: %+v", err)
			return err
		}
		totalBytesWritten += bytesWritten
		if state.writer.Available() <= 0 {
			if err = state.writer.Flush(); err != nil {
				log.Errorf("tcp stream flush buffer immediately err:", err.Error())
				return err
			}
		}
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

func sendMessage(state *ConnState, message proto.Message) error {
	bytes, err := proto.Marshal(prepareMessage(message, state))
	if err != nil {
		return errors.Wrap(err, "failed to marshal message in TCP sendMessage")
	}
	if len(bytes) == 0 {
		log.Error("stack info:", fmt.Sprintf("%s", debug.Stack()))
		return errors.New("tcp sendMessage,len(message) is empty")
	}

	var enable uint16
	if common.Parameters.Compression.Enable {
		enable = 1
	} else {
		enable = 0
	}
	// Serialize size.
	buffer := make([]byte, 10)
	binary.BigEndian.PutUint32(buffer, common.Parameters.NetworkID)
	binary.BigEndian.PutUint16(buffer[4:], uint16(enable<<8)|(uint16(common.Parameters.Compression.CompressAlgo)&0xFF))
	binary.BigEndian.PutUint32(buffer[6:], uint32(len(bytes)))

	buffer = append(buffer, bytes...)

	// Write until all bytes have been written.
	bytesWritten, totalBytesWritten := 0, 0

	state.writerMutex.Lock()
	defer state.writerMutex.Unlock()

	for totalBytesWritten < len(buffer) && err == nil {
		bytesWritten, err = state.writer.Write(buffer[totalBytesWritten:])
		if err != nil {
			log.Errorf("tcp stream(common): failed to write entire buffer, err: %+v", err)
			return err
		}
		totalBytesWritten += bytesWritten
		if state.writer.Available() <= 0 {
			if err = state.writer.Flush(); err != nil {
				log.Errorf("tcp stream flush buffer immediately err:", err.Error())
				return err
			}
		}
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
		if state.writer.Available() <= 0 {
			if err = state.writer.Flush(); err != nil {
				log.Error("stream flush err in buffer immediately written:", err.Error())
				break
			}
		}
	}

	if err != nil {
		return errors.Errorf("(tcp) stream: failed to write to socket,err:%s", err.Error())
	}

	if err := state.writer.Flush(); err != nil {
		log.Errorf("tcp stream(raw): failed to flush entire buffer, err: %+v", err)
		return err
	}

	return nil
}
