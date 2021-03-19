package main

import (
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/lesismal/nbio/nbhttp"
	"github.com/lesismal/nbio/nbhttp/websocket"
)

var (
	qps   uint64 = 0
	total uint64 = 0
)

func onEcho(w http.ResponseWriter, r *http.Request) {
	data, _ := io.ReadAll(r.Body)
	if len(data) > 0 {
		w.Write(data)
	} else {
		w.Write([]byte(time.Now().Format("20060102 15:04:05")))
	}
	atomic.AddUint64(&qps, 1)
}

func onWebsocket(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.NewUpgrader()
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		panic(err)
	}
	wsConn := conn.(*websocket.Conn)
	wsConn.SetMessageHandler(func(messageType int8, data []byte) {
		fmt.Println("+++ onMessage:", messageType, string(data))
		wsConn.WriteMessage(messageType, data)
	})
	fmt.Println("on ws conn:", wsConn.RemoteAddr().String())
}

func main() {
	mux := &http.ServeMux{}
	mux.HandleFunc("/echo", onEcho)
	mux.HandleFunc("/ws", onWebsocket)

	svr := nbhttp.NewServer(nbhttp.Config{
		Network: "tcp",
		Addrs:   []string{"localhost:28000"},
	}, mux, nil, nil)

	err := svr.Start()
	if err != nil {
		fmt.Printf("nbio.Start failed: %v\n", err)
		return
	}
	defer svr.Stop()

	ticker := time.NewTicker(time.Second)
	for i := 1; true; i++ {
		<-ticker.C
		// n := atomic.SwapUint64(&qps, 0)
		// total += n
		// fmt.Printf("running for %v seconds, online: %v, NumGoroutine: %v, qps: %v, total: %v\n", i, svr.State().Online, runtime.NumGoroutine(), n, total)
	}
}
