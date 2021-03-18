package main

import (
	"log"
	"net"
	"time"

	"github.com/lesismal/arpc"
	"github.com/lesismal/arpc/extension/protocol/websocket"
)

func main() {
	arpc.DefaultHandler.Handle("/server/notify", func(ctx *arpc.Context) {
		str := ""
		err := ctx.Bind(&str)
		log.Printf("/server/notify: \"%v\", error: %v", str, err)
	})

	client, err := arpc.NewClient(func() (net.Conn, error) {
		return websocket.Dial("ws://localhost:28000/ws")
	})
	if err != nil {
		panic(err)
	}
	defer client.Stop()

	req := "hello"
	rsp := ""
	err = client.Call("/call/echo", &req, &rsp, time.Second*5)
	if err != nil {
		log.Fatalf("Call failed: %v", err)
	} else {
		log.Printf("Call Response: \"%v\"", rsp)
	}
}
