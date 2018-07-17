// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Test throughput with small stacks and high allocation rate.
//
// TODO: What metrics? Allocation rate?
package main

import (
	"flag"
	"os"
	"runtime"
	"time"
	"fmt"

	"github.com/aclements/go-gcbench/gcbench"
)

var sink []*byte

var flagDuration = flag.Duration("benchtime", 10*time.Second, "steady state duration")

func main() {
	memstats := new(runtime.MemStats)
	start := time.Now()
	flag.Parse()
	if flag.NArg() > 0 {
		flag.Usage()
		os.Exit(2)
	}

	gcbench.NewBenchmark("SmallHeap", benchMain).Run()
	elapsed := time.Since(start)
	fmt.Print("time: ", elapsed)
	printMemStats(memstats)
}

func benchMain() {
	sink = make([]*byte, 4*runtime.GOMAXPROCS(-1))
	for i := range sink {
		go func(i int) {
			for {
				sink[i] = &make([]byte, 64)[0]
			}
		}(i)
	}

	<-time.After(*flagDuration)
	os.Exit(0)
}

func printMemStats(memstats *runtime.MemStats) {
   runtime.ReadMemStats(memstats)
   fmt.Print(" | TotalAlloc ", memstats.TotalAlloc)
   fmt.Print(" | mallocs ", memstats.Mallocs)
   fmt.Print(" | frees ", memstats.Mallocs - memstats.Frees)
   fmt.Println(" | GC cycles ", memstats.NumGC)
}

