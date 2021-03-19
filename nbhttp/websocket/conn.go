package websocket

import (
	"encoding/binary"
	"fmt"
	"net"
	"sync"

	"github.com/lesismal/nbio/mempool"
)

const (
	maxFrameHeaderSize         = 14
	maxControlFramePayloadSize = 125
	framePayloadSize           = 4096 - maxFrameHeaderSize
)

// The message types are defined in RFC 6455, section 11.8.
const (
	TextMessage   int8 = 1
	BinaryMessage int8 = 2
	CloseMessage  int8 = 8
	PingMessage   int8 = 9
	PongMessage   int8 = 10
)

type Conn struct {
	net.Conn

	mux sync.Mutex

	// compress         bool
	// compressionLevel int
	subprotocol string
	readLimit   int64

	pingHandler    func(appData string)
	pongHandler    func(appData string)
	messageHandler func(messageType int8, data []byte)
	closeHandler   func(code int, text string)
}

func (c *Conn) SetReadLimit(limit int64) {
	c.readLimit = limit
}

func (c *Conn) handleMessage(opcode int8, data []byte) {
	switch opcode {
	case TextMessage, BinaryMessage:
		c.messageHandler(opcode, data)
	case CloseMessage:
		if len(data) >= 2 {
			code := int(binary.BigEndian.Uint16(data[:2]))
			c.closeHandler(code, string(data[2:]))
		} else {
			c.WriteMessage(CloseMessage, nil)
		}
	case PingMessage:
		c.pingHandler(string(data))
	case PongMessage:
		c.pongHandler(string(data))
	default:
	}
	fmt.Printf("+++ HandleMessage, opcode: %v, message: %v\n", opcode, len(data))
}

func (c *Conn) SetCloseHandler(h func(code int, text string)) {
	if h != nil {
		c.closeHandler = h
	}
}

func (c *Conn) SetPingHandler(h func(appData string)) {
	if h != nil {
		c.pingHandler = h
	}
}

func (c *Conn) SetPongHandler(h func(appData string)) {
	if h != nil {
		c.pongHandler = h
	}
}

func (c *Conn) SetMessageHandler(h func(messageType int8, data []byte)) {
	if h != nil {
		c.messageHandler = h
	}
}

// func (c *Conn) EnableWriteCompression(enable bool) {
// 	c.compress = enable
// }

// func (c *Conn) SetCompressionLevel(level int) error {
// 	if !isValidCompressionLevel(level) {
// 		return errors.New("websocket: invalid compression level")
// 	}
// 	c.compressionLevel = level
// 	return nil
// }

func (c *Conn) writeMessage(messageType int8, fin bool, data []byte) error {
	var (
		buf     []byte
		bodyLen = len(data)
		offset  = 2
	)
	if bodyLen < 126 {
		buf = mempool.Malloc(len(data) + 2)
		buf[1] = byte(bodyLen)
		// offset = 2
	} else if bodyLen < 65535 {
		buf = mempool.Malloc(len(data) + 4)
		buf[1] = 126
		binary.BigEndian.PutUint16(buf[2:4], uint16(bodyLen))
		offset = 4
	} else {
		buf = mempool.Malloc(len(data) + 10)
		buf[1] = 127
		binary.BigEndian.PutUint64(buf[2:10], uint64(bodyLen))
		offset = 10
	}
	copy(buf[offset:], data)

	// opcode
	buf[0] = byte(messageType)

	// fin
	if fin {
		buf[0] |= byte(0x80)
	}

	// mask
	// buf[1] |= byte(0x80)
	// rand.Read(buf[offset : offset+4])
	// mask := buf[offset : offset+4]
	// // body
	// for i := 0; i < bodyLen; i++ {
	// 	buf[offset+i] = (data[i] ^ mask[i%4])
	// }
	fmt.Printf("--- WriteMessage buf[0]: %b, buf[1]: %b\n", buf[0], buf[1])
	_, err := c.Conn.Write(buf)
	return err
}

func (c *Conn) WriteMessage(messageType int8, data []byte) error {
	c.mux.Lock()
	defer c.mux.Unlock()

	switch messageType {
	case TextMessage:
	case BinaryMessage:
	case PingMessage, PongMessage, CloseMessage:
		if len(data) > maxControlFramePayloadSize {
			return ErrInvalidControlFrame
		}
	default:
	}

	for len(data) > 0 {
		n := len(data)
		if n > framePayloadSize {
			n = framePayloadSize
		}
		err := c.writeMessage(messageType, n == len(data), data[:n])
		if err != nil {
			return err
		}
		data = data[n:]
	}

	return nil
}

func (c *Conn) Write(data []byte) (int, error) {
	return -1, ErrInvalidWriteCalling
}

func newConn(c net.Conn, compress bool, subprotocol string) *Conn {
	conn := &Conn{
		Conn: c,
		// compress:    compress,
		subprotocol:    subprotocol,
		pongHandler:    func(string) {},
		messageHandler: func(int8, []byte) {},
	}
	conn.pingHandler = func(message string) {
		conn.WriteMessage(PongMessage, nil)
	}
	conn.closeHandler = func(code int, text string) {
		if len(text)+2 > maxControlFramePayloadSize {
			return //ErrInvalidControlFrame
		}
		buf := mempool.Malloc(len(text) + 2)
		binary.BigEndian.PutUint16(buf[:2], uint16(code))
		copy(buf[2:], text)
		conn.WriteMessage(CloseMessage, buf)
		mempool.Free(buf)
	}
	return conn
}
