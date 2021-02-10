// +build linux darwin netbsd freebsd openbsd dragonfly

package nbio

import (
	"log"
	"runtime"
	"strings"
	"syscall"
)

// Start init and start pollers
func (g *Gopher) Start() error {
	var err error

	g.lfds = []int{}

	for _, addr := range g.addrs {
		fd, err := listen(g.network, addr, g.maxLoad)
		if err != nil {
			return err
		}

		g.lfds = append(g.lfds, fd)
	}

	for i := uint32(0); i < g.listenerNum; i++ {
		g.listeners[i], err = newPoller(g, true, int(i))
		if err != nil {
			for j := 0; j < int(i); j++ {
				syscall.Close(g.lfds[j])
				g.listeners[j].stop()
			}
			return err
		}
	}

	for i := uint32(0); i < g.pollerNum; i++ {
		g.pollers[i], err = newPoller(g, false, int(i))
		if err != nil {
			for j := 0; j < int(len(g.lfds)); j++ {
				syscall.Close(g.lfds[j])
				g.listeners[j].stop()
			}

			for j := 0; j < int(i); j++ {
				g.pollers[j].stop()
			}
			return err
		}
	}

	for i := uint32(0); i < g.pollerNum; i++ {
		g.pollers[i].readBuffer = make([]byte, g.readBufferSize)
		g.Add(1)
		go g.pollers[i].start()
	}
	for _, l := range g.listeners {
		g.Add(1)
		go l.start()
	}

	if len(g.addrs) == 0 {
		log.Printf("gopher start")
	} else {
		log.Printf("gopher start listen on: [\"%v\"]", strings.Join(g.addrs, `", "`))
	}
	return nil
}

// NewGopher is a factory impl
func NewGopher(conf Config) *Gopher {
	cpuNum := uint32(runtime.NumCPU())
	if conf.MaxLoad == 0 {
		conf.MaxLoad = DefaultMaxLoad
	}
	if len(conf.Addrs) > 0 && conf.NListener == 0 {
		conf.NListener = 1
	}
	if conf.NPoller == 0 {
		conf.NPoller = cpuNum
	}
	if conf.ReadBufferSize == 0 {
		conf.ReadBufferSize = DefaultReadBufferSize
	}

	g := &Gopher{
		network:            conf.Network,
		addrs:              conf.Addrs,
		maxLoad:            int64(conf.MaxLoad),
		listenerNum:        conf.NListener,
		pollerNum:          conf.NPoller,
		readBufferSize:     conf.ReadBufferSize,
		maxWriteBufferSize: conf.MaxWriteBufferSize,
		listeners:          make([]*poller, conf.NListener),
		pollers:            make([]*poller, conf.NPoller),
		conns:              map[*Conn][]byte{},
		connsLinux:         make([]*Conn, conf.MaxLoad+64),
		onOpen:             func(c *Conn) {},
		onClose:            func(c *Conn, err error) {},
		onData:             func(c *Conn, data []byte) {},
	}

	return g
}
