package websocket

import (
	"net"
)

type Conn struct {
	net.Conn
	compress    bool
	subprotocol string
}

func newConn(c net.Conn, compress bool, subprotocol string) *Conn {
	return &Conn{
		Conn:        c,
		compress:    compress,
		subprotocol: subprotocol,
	}
}
