package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"log"
	"runtime/pprof"
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

func controlclients(clients_finished chan bool, s host.Host, numwaitgroups *int, delay int, numclients int, timeout int) {
	var newclient_calls, numclient_goroutines int
	var clients_done int32
	//var delay float64
	var completed map[int]bool
	completed = make(map[int]bool, 1000)
	err := logging.SetLogLevel("basichost", "debug")
	if err != nil {
		panic(err)
	}
	/*lvl, err := logging.LevelFromString("error")
	if err != nil {
		panic(err)
	}
	logging.SetAllLoggers(lvl)*/
	var mu sync.Mutex
	n := atomic.Int32{}
	wg := sync.WaitGroup{}
	for i := 0; i < numclients; i++ {
		if i%delay == 0 {
			time.Sleep(time.Duration(1) * 10 * time.Millisecond)
			//delay = float64(delayParam) * 1000
			//fmt.Printf("Std Delay %f\n", delay)
		} else {
			time.Sleep(time.Duration(i%delay) * 10 * time.Millisecond)
			//delay = float64(i%delayParam) * 1000
			//fmt.Printf("Mod Delay %f\n", delay)
		}
		wg.Add(1)
		*numwaitgroups++
		go func(i int) {
			mu.Lock()
			completed[i] = false
			mu.Unlock()
			numclient_goroutines++
			defer wg.Done()
			newclient_calls++
			err := newClient(peer.AddrInfo{
				ID:    s.ID(),
				Addrs: s.Addrs(),
			})
			newclient_calls--
			if err != nil {
				panic(err)
			}
			//println("Clients done:", n.Add(1))
			// println(i)
			mu.Lock()
			delete(completed, i)
			mu.Unlock()
			*numwaitgroups--
			numclient_goroutines--
			clients_done = n.Add(1)
			fmt.Printf("Loop %d,  Clients done: %d, new client calls outstanding %d, client goroutines %d, numwait groups %d\n", i, clients_done, newclient_calls, numclient_goroutines, *numwaitgroups)
			if clients_done == int32(numclients) {
				clients_finished <- true
			}
		}(i)
	}

	//wg.Wait()
	//clients_finished <- true
	select {
	case <-time.After(time.Duration(timeout) * time.Second):
		fmt.Printf("%d second timeout\n", timeout)
	case signal := <-clients_finished:
		fmt.Printf("clients finished %v\n", signal)
	}
	for j := 0; j < numclients; j++ {
		_, ok := completed[j]
		if ok {
			fmt.Printf("Didn't complete %d\n", j)
		}
	}
}

// func CreateClients(t *testing.T) {
func main() {
	var clients_finished chan bool
	var numwaitgroups int
	delay := flag.Int("delay", 100, "delay parameters")
	numclients := flag.Int("numclients", 1000, "number of clients")
	timeout := flag.Int("timeout", 180, "timeout (seconds)")
	flag.Parse()
	fmt.Printf("Delay Param %d, Number of clients %d, Timeout (seconds) %d\n", *delay, *numclients, *timeout)
	f, err := os.Create("cpuprofile.prof")
	if err != nil {
		log.Fatal(err)
	}
	pprof.StartCPUProfile(f)
	defer pprof.StopCPUProfile()
	if err != nil {
		panic("Needs parameter as divisor for time delay e.g. 100")
	}
	clients_finished = make(chan bool, 1)
	limits := rcmgr.PartialLimitConfig{
		System: rcmgr.ResourceLimits{
			Conns:        1024,
			ConnsInbound: 1024,
		},
		Transient: rcmgr.ResourceLimits{
			Conns:        1024,
			ConnsInbound: 1024,
		},
	}

	limiter := rcmgr.NewFixedLimiter(limits.Build(rcmgr.DefaultLimits.AutoScale()))
	rmgr, err := rcmgr.NewResourceManager(limiter)
	if err != nil {
		panic(err)
	}

	cmgr, err := connmgr.NewConnManager(0, 1024)
	if err != nil {
		panic(err)
	}

	s, err := libp2p.New(libp2p.ResourceManager(rmgr), libp2p.ConnectionManager(cmgr))
	if err != nil {
		panic(err)
	}
	controlclients(clients_finished, s, &numwaitgroups, *delay, *numclients, *timeout)

	fmt.Printf("Done Connections:%d number of waitgroups %d\n", len(s.Network().Conns()), numwaitgroups) // Often more than 1000 because one peer may have succeeded in opening 2 connections at once.

}

func newClient(addrInfo peer.AddrInfo) error {
	// client
	ctx := context.Background()
	c, err := libp2p.New(libp2p.NoListenAddrs)
	if err != nil {
		return err
	}

	err = c.Connect(ctx, addrInfo)
	if err != nil {
		return err
	}

	p := ping.NewPingService(c)
	res := <-p.Ping(ctx, addrInfo.ID)

	if res.Error != nil {
		return res.Error
	}

	return nil
}
