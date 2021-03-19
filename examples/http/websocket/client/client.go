package main

import (
	"flag"
	"log"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
)

var addr = flag.String("addr", "localhost:28001", "http service address")

func main() {
	flag.Parse()

	u := url.URL{Scheme: "ws", Host: *addr, Path: "/ws"}
	log.Printf("connecting to %s", u.String())

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer c.Close()

	go func() {
		text := "hello world"
		for {
			err := c.WriteMessage(websocket.TextMessage, []byte(text))
			if err != nil {
				log.Fatalf("write: %v", err)
				return
			}
			log.Println("write:", text)

			_, message, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				return
			}
			if string(message) != text {
				log.Fatalf("message != text: %v, %v", len(message), string(message))
			} else {
				log.Println("read :", string(message))
			}
			time.Sleep(time.Second)
		}
	}()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	<-interrupt
	log.Println("interrupt")
}
