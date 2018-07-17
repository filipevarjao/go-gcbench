// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Test mutator availability and end-to-end latency of an RPC-like system.
package main

// TODO: This benchmark doesn't target a particular aspect of the GC
// like other gcbench benchmarks do. It would be nice if we just had
// composable sets of metrics that a benchmark could opt in to without
// needing a whole "framework", but I'm not sure how to do that with
// the gctrace on stderr thing. Maybe we could dup our own FD over
// stderr to read our own gctrace? That's dangerous if something goes
// wrong since you'll lose the panic. We could pipe it by way of an
// external program that could report if something goes wrong (e.g.,
// tee if you just want to see the whole log). This might also make it
// easier to ignore initialization and warm-up in the GC log.

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aclements/go-gcbench/gcbench"
	"github.com/aclements/go-gcbench/gcbench/heapgen"
)

var (
	flagDuration = flag.Duration("benchtime", 10*time.Second, "steady state duration")
	flagBallast  = gcbench.FlagBytes("ballast", 64*gcbench.MB, "retain `x` bytes of global data")
	// XXX Make this reqs-per-sec-per-p so it scales properly.
	flagReqsPerSec = flag.Float64("reqs-per-sec", 8000, "send `rate` requests per second")
	flagClient     = flag.String("client", "", "internal flag to act as RPC client")
)

func main() {
	memstats := new(runtime.MemStats)
	start := time.Now()
	flag.Parse()
	if flag.NArg() > 0 {
		flag.Usage()
		os.Exit(2)
	}

	if *flagClient != "" {
		doClient(*flagClient)
		return
	}

	name := "RPC"
	gcbench.NewBenchmark(name, benchMain).Config("reqs-per-sec", *flagReqsPerSec).Config("ballast", *flagBallast).Run()
	elapsed := time.Since(start)
	fmt.Print("time: ", elapsed)
	printMemStats(memstats)

}

var sink1 interface{}
var requestCount int64
var serverLatency gcbench.LatencyDist

func benchMain() {
	// Create the ballast.
	m := heapgen.Measure(heapgen.MakeAST)
	sink1 = heapgen.Generate(m.Gen, m.BytesRetained, int(*flagBallast))

	// Divide GOMAXPROCS by two so it's split between client and
	// server.
	gomaxprocs := runtime.GOMAXPROCS(-1)
	runtime.GOMAXPROCS(gomaxprocs / 2)

	// Start the server.
	l := startServer()

	// Start the client.
	client := exec.Command(os.Args[0], "-client", l.Addr().String(), "-reqs-per-sec", fmt.Sprint(*flagReqsPerSec))
	client.Env = []string{"GODEBUG=gctrace=1", "GOGC=off",
		fmt.Sprintf("GOMAXPROCS=%d", gomaxprocs/2)}
	cin, err := client.StdinPipe()
	if err != nil {
		log.Fatal("creating client stdin pipe: ", err)
	}
	defer cin.Close()
	cout, err := client.StdoutPipe()
	if err != nil {
		log.Fatal("creating client stdout pipe: ", err)
	}
	defer cout.Close()
	var clientOut bytes.Buffer
	client.Stderr = &clientOut
	err = client.Start()
	if err != nil {
		log.Fatal("starting client: ", err)
	}

	// Run the server.
	done := make(chan bool)
	startTime := time.Now()
	time.AfterFunc(*flagDuration, func() {
		gcbench.ReportExtra("reqs/sec", float64(atomic.LoadInt64(&requestCount))/float64(time.Since(startTime))*float64(time.Second))

		// Server-reported latency.
		fmt.Fprintf(os.Stderr, "server-measured request latency:\n")
		serverLatency.FprintHist(os.Stderr, 70, 5)

		// Shut down client.
		cin.Close()
		err := client.Wait()
		fmt.Fprintf(os.Stderr, "client stderr:\n%s", clientOut.String())
		if err != nil {
			log.Fatal("client failed: ", err)
		}
		close(done)
	})
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				log.Fatal("accept: ", err)
			}
			go handler(c)
		}
	}()
	<-done
}

func startServer() net.Listener {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		if l, err = net.Listen("tcp6", "[::1]:0"); err != nil {
			log.Fatalf("failed to listen: %v", err)
		}
	}
	return l
}

func handler(c net.Conn) {
	var header [1]byte
	r, w := bufio.NewReader(c), bufio.NewWriter(c)
	d := json.NewDecoder(r)
	for {
		// Read the request.
		n, err := r.Read(header[:])
		if err == io.EOF {
			c.Close()
			return
		}
		lt := serverLatency.Start()
		if n != 1 || header[0] != 'x' {
			log.Fatal("bad header: ", n, err, header)
		}

		var data interface{}
		err = d.Decode(&data)
		if err != nil {
			log.Fatal("decoding request: ", err)
		}

		atomic.AddInt64(&requestCount, 1)

		// Send the response.
		json.NewEncoder(w).Encode(data)
		w.Flush()
		lt.Tick()
	}
}

func doClient(addr string) {
	var wg sync.WaitGroup
	stop := make(chan bool)

	var lat gcbench.LatencyDist

	// Wait for stdin to close, then shut down cleanly.
	go func() {
		var buf [1024]byte
		for {
			n, err := os.Stdin.Read(buf[:])
			if n <= 0 || err != nil {
				break
			}
		}
		log.Print("cleaning up client")
		close(stop)
		wg.Wait()
		log.Print("closing ", len(conns.q), " connections")
		for _, c := range conns.q {
			c.Close()
		}
		fmt.Fprintln(os.Stderr, "client-measured request latency:")
		lat.FprintHist(os.Stderr, 70, 5)
		gcbench.ReportExtra("P99-latency-ns", float64(lat.Quantile(0.99)))
		gcbench.ReportExtra("P99.9-latency-ns", float64(lat.Quantile(0.999)))
		gcbench.ReportExtra("max-latency-ns", float64(lat.Max))
		//log.Print("client exiting")
		os.Exit(0)
	}()

	// Generate a Poisson process by spacing events out by an
	// exponential distribution and using open-loop control. To
	// minimize the bottleneck in the coordinator, we shard this
	// process across several goroutines.
	const shards = 16
	reqsPerNSPerShard := *flagReqsPerSec / 1e9 / shards
	for i := 0; i < shards; i++ {
		wg.Add(1)
		go func() {
		loop:
			for {
				wait := rand.ExpFloat64() / reqsPerNSPerShard
				select {
				case <-stop:
					break loop
				case <-time.After(time.Duration(wait)):
				}
				wg.Add(1)
				go sendOneRequest(addr, &wg, &lat)
			}
			wg.Done()
		}()
	}
	select {}
}

var reqBytes = []byte(`x{
	"Dir": "src/runtime",
	"ImportPath": "runtime",
	"Name": "runtime",
	"Doc": "Package runtime contains operations that interact with Go's runtime system, such as functions to control goroutines.",
	"Target": "pkg/linux_amd64/runtime.a",
	"Goroot": true,
	"Standard": true,
	"Stale": true,
	"StaleReason": "stale dependency",
	"Root": "go.dev"
}`)

var conns struct {
	q []net.Conn
	sync.Mutex
}

func sendOneRequest(addr string, wg *sync.WaitGroup, lat *gcbench.LatencyDist) {
	defer wg.Done()

	var c net.Conn
	conns.Lock()
	if len(conns.q) > 0 {
		c = conns.q[len(conns.q)-1]
		conns.q = conns.q[:len(conns.q)-1]
	}
	conns.Unlock()
	if c == nil {
		var err error
		c, err = net.Dial("tcp", addr)
		if err != nil {
			// This could just mean the server is
			// overloaded. (TODO: Maybe it's this single
			// Accept that gets overloaded?)
			log.Print("dialing ", addr, ": ", err)
			return
		}
	}

	// Send the request.
	//log.Print("sending request", c)
	lt := lat.Start()
	n, err := c.Write(reqBytes)
	if err != nil {
		log.Fatal("writing request: ", err)
	}
	if n != len(reqBytes) {
		log.Fatal("short write")
	}
	//log.Print("sent request; reading response", c)

	// Read the response.
	var buf [1024]byte
	var done bool
	for !done {
		n, err := c.Read(buf[:])
		if bytes.IndexRune(buf[:n], '\n') >= 0 {
			done = true
		}
		if err == io.EOF {
			c.Close()
			return
		}
		if err != nil {
			log.Fatal("reading response: ", err)
		}
	}
	lt.Tick()
	//log.Print("read response", c)

	conns.Lock()
	conns.q = append(conns.q, c)
	conns.Unlock()
}

func printMemStats(memstats *runtime.MemStats) {
   runtime.ReadMemStats(memstats)
   fmt.Print(" | TotalAlloc ", memstats.TotalAlloc)
   fmt.Print(" | mallocs ", memstats.Mallocs)
   fmt.Print(" | frees ", memstats.Mallocs - memstats.Frees)
   fmt.Println(" | GC cycles ", memstats.NumGC)
}
