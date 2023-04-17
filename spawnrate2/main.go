package main

import (
	"context"
	"flag"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	logging "github.com/ipfs/go-log/v2"
	libp2p "github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	"github.com/libp2p/go-libp2p/p2p/protocol/ping"
)

func setupLogging() {
	err := logging.SetLogLevel("ping", "debug")
	if err != nil {
		panic(err)
	}
	err = logging.SetLogLevel("upgrader", "debug")
	if err != nil {
		panic(err)
	}
	err = logging.SetLogLevel("quic-transport", "debug")
	if err != nil {
		panic(err)
	}
	err = logging.SetLogLevel("webtransport", "debug")
	if err != nil {
		panic(err)
	}

}

func controlclients(s host.Host, clients int) {
	n := atomic.Int32{}
	wg := sync.WaitGroup{}
	for i := 0; i < clients; i++ {
		wg.Add(1)
		go func(i int) {
			time.Sleep(time.Duration(i%100) * 100 * time.Millisecond)
			defer wg.Done()
			err := newClient(peer.AddrInfo{
				ID:    s.ID(),
				Addrs: s.Addrs(),
			})
			if err != nil {
				panic(err)
			}
			println("Clients done:", n.Add(1))
			// println(i)
		}(i)
	}

	wg.Wait()

}

func main() {
	var s host.Host
	setupLogging()
	setip := flag.Bool("setip", false, "Set default IP or leave to stated defaults")
	clients := flag.Int("clients", 1000, "Set number of clients")
	flag.Parse()
	fmt.Printf("Running with setip=%v and number of clients %d\n", *setip, *clients)
	limits := rcmgr.PartialLimitConfig{
		System: rcmgr.ResourceLimits{
			Streams:         rcmgr.LimitVal((*clients) * 2),
			StreamsInbound:  rcmgr.LimitVal((*clients) * 2),
			StreamsOutbound: rcmgr.LimitVal((*clients) * 2),
			Conns:           rcmgr.LimitVal((*clients) * 2),
			ConnsInbound:    rcmgr.LimitVal((*clients) * 2),
			ConnsOutbound:   rcmgr.LimitVal((*clients) * 2),
		},
		Transient: rcmgr.ResourceLimits{
			Streams:         rcmgr.LimitVal((*clients) * 2),
			StreamsInbound:  rcmgr.LimitVal((*clients) * 2),
			StreamsOutbound: rcmgr.LimitVal((*clients) * 2),
			Conns:           rcmgr.LimitVal((*clients) * 2),
			ConnsInbound:    rcmgr.LimitVal((*clients) * 2),
			ConnsOutbound:   rcmgr.LimitVal((*clients) * 2),
		},
	}

	limiter := rcmgr.NewFixedLimiter(limits.Build(rcmgr.DefaultLimits.AutoScale()))
	rmgr, err := rcmgr.NewResourceManager(limiter)
	if err != nil {
		panic(err)
	}

	cmgr, err := connmgr.NewConnManager(0, *clients)
	if err != nil {
		panic(err)
	}
	if *setip {
		s, err = libp2p.New(libp2p.ResourceManager(rmgr), libp2p.ConnectionManager(cmgr), libp2p.ListenAddrStrings("/ip4/127.0.0.1/udp/0/quic-v1"))
	} else {
		s, err = libp2p.New(libp2p.ResourceManager(rmgr), libp2p.ConnectionManager(cmgr))
	}

	if err != nil {
		panic(err)
	}
	controlclients(s, *clients)
	fmt.Println("Done Connections:", len(s.Network().Conns())) // Often more than 1000 because one peer may have succeeded in opening 2 connections at once.
}

func newClient(addrInfo peer.AddrInfo) error {
	// client
	var err error
	ctx := context.Background()
	c, err := libp2p.New(libp2p.NoListenAddrs)
	if err != nil {
		fmt.Println("Error lip2pNew")
		return err
	}

	err = c.Connect(ctx, addrInfo)
	if err != nil {
		fmt.Println("Connection Error")
		return err
	}

	p := ping.NewPingService(c)
	res := <-p.Ping(ctx, addrInfo.ID)

	if res.Error != nil {
		fmt.Printf("Ping Error %v\n", addrInfo.ID)
		return res.Error
	}
	//c.Close()
	return nil
}
