package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"log"
	"strings"
	"time"

	"github.com/lesismal/nbio"
)

func main() {

	var (
		ret    []byte
		buf    = []byte("nbio: hello tls")
		addr   = "localhost:8888"
		ctx, _ = context.WithTimeout(context.Background(), time.Second)

		tlsConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
	)

	g := nbio.NewGopher(nbio.Config{})

	done := make(chan int)
	g.OnOpen(func(c *nbio.Conn) {
		tlsConn := tls.Client(c, tlsConfig)
		c.SetSession(tlsConn)
		tlsConn.Write(buf)
	})
	g.OnRead(func(c *nbio.Conn, b []byte) ([]byte, error) {
		tlsConn := c.Session().(*tls.Conn)
		n, err := tlsConn.Read(b)
		log.Println("tlsConn.Read client:", n, err)
		if err != nil {
			if strings.HasPrefix(err.Error(), "tls:") {
				if n > 0 {
					return b[:n], nil
				}
				return nil, nil
			}
			if err != nil {
				return nil, err
			}
			return b[:n], nil
		}
		return b[:n], err
	})
	g.OnData(func(c *nbio.Conn, data []byte) {
		if len(data) <= 0 {
			return
		}
		ret = append(ret, data...)
		if len(ret) == len(buf) {
			if bytes.Equal(buf, ret) {
				close(done)
			}
		}
	})

	err := g.Start()
	if err != nil {
		log.Printf("Start failed: %v\n", err)
	}
	defer g.Stop()

	c, err := nbio.Dial("tcp", addr)
	if err != nil {
		log.Printf("Dial failed: %v\n", err)
	}
	g.AddConn(c)

	select {
	case <-ctx.Done():
		log.Fatal("timeout")
	case <-done:
		log.Println("success")
	}
}
