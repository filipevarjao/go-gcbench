// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Test STW latency and mark throughput on large, heterogeneous heaps.
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/aclements/go-gcbench/gcbench"
	"github.com/aclements/go-gcbench/gcbench/heapgen"
)

var sink1, sink2 interface{}

var (
	flagDuration = flag.Duration("benchtime", 20*time.Second, "steady state duration")
	flagRetain   = gcbench.FlagBytes("retain", gcbench.GB, "retain `x` bytes of heap")
	flagHeap     = flag.String("heap", "AST", "heap `shape`; either \"AST\" or \"deBruijn2\"")
	flagSTW      = flag.Bool("stw", false, "use STW GC")
)

var heapMaker func() interface{}

func main() {
	flag.Parse()
	if flag.NArg() > 0 {
		flag.Usage()
		os.Exit(2)
	}
	switch *flagHeap {
	case "AST":
		heapMaker = heapgen.MakeAST
	case "deBruijn2":
		heapMaker = func() interface{} {
			// About 1MB per graph.
			return heapgen.MakeDeBruijn2(16)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown -heap shape %q\n", *flagHeap)
		flag.Usage()
		os.Exit(2)
	}

	name := "LargeHeap"
	if *flagSTW {
		// This is a fairly different benchmark.
		name += "STW"
	}
	gcbench.NewBenchmark(name, benchMain).Config("retain", *flagRetain).Config("heap", *flagHeap).Run()
}

func benchMain() {
	m := heapgen.Measure(heapMaker)
	println(m.BytesRetained, "bytes per graph")
	sink1 = heapgen.Generate(m.Gen, m.BytesRetained, int(*flagRetain))

	// TODO: Report more of the allocation time distribution.

	// On my laptop for 1.5 and 1.6, this takes another ~10
	// seconds to reach steady state.
	var latDist gcbench.LatencyDist
	time.AfterFunc(*flagDuration, func() {
		maxLat := atomic.LoadInt64((*int64)(&latDist.Max))
		gcbench.ReportExtra("max-latency-ns", float64(maxLat))
		os.Exit(0)
	})
	lat := latDist.Start()
	for {
		if *flagSTW {
			runtime.GC()
		} else {
			sink2 = m.Gen()
		}
		lat.Tick()
	}
	lat.Done() // For completeness.
}
