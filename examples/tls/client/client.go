package main

import (
	"log"
	"strings"

	"github.com/lesismal/lib/std/crypto/tls"
	"github.com/lesismal/nbio"
)

func main() {

	var (
		buf       = []byte("nbio: hello tls")
		addr      = "localhost:8888"
		tlsConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
	)

	g := nbio.NewGopher(nbio.Config{})

	g.OnOpen(func(c *nbio.Conn) {
		tlsConn := tls.NewConn(c, tlsConfig, true, true, 0)
		c.SetSession(tlsConn)
		tlsConn.Write(buf)
	})
	g.OnRead(func(c *nbio.Conn, b []byte) ([]byte, error) {
		tlsConn := c.Session().(*tls.Conn)
		n, err := tlsConn.Read(b)
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

		log.Println("onData:", string(data))
		tlsConn := c.Session().(*tls.Conn)
		tlsConn.Write(append([]byte{}, data...))
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

	g.Wait()
}
