// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Test STW latency and on large BSSes.
package main

import (
	"flag"
	"os"
	"time"
	"fmt"
	"runtime"

	"github.com/aclements/go-gcbench/gcbench"
)

const ptrSize = 4 << (^uintptr(0) >> 63)

const bss = 100 * gcbench.MB

// Pessimal case:
var thing [bss / (2 * ptrSize)]struct {
	x [ptrSize]byte
	y *byte
}

// Optimal case:
// var thing [1]struct {
// 	x [bssBytes - ptrSize]byte
// 	y *byte
// }

var sink interface{}

var flagDuration = flag.Duration("benchtime", 10*time.Second, "steady state duration")

func main() {
	memstats := new(runtime.MemStats)
	start := time.Now()

	gcbench.NewBenchmark("LargeBSS", benchMain).Config("bss", bss).Run()
	elapsed := time.Since(start)
	fmt.Print("time: ", elapsed)
	printMemStats(memstats)
}

func benchMain() {
	flag.Parse()

	for i := range thing {
		thing[i].y = nil
	}

	check := gcbench.NewGCChecker()
	time.AfterFunc(*flagDuration, func() {
		check.NumGCAtLeast(10)
		os.Exit(0)
	})
	for {
		sink = make([]byte, 1<<10)
	}
}

func printMemStats(memstats *runtime.MemStats) {
   runtime.ReadMemStats(memstats)
   fmt.Print(" | TotalAlloc ", memstats.TotalAlloc)
   fmt.Print(" | mallocs ", memstats.Mallocs)
   fmt.Print(" | frees ", memstats.Mallocs - memstats.Frees)
   fmt.Println(" | GC cycles ", memstats.NumGC)
}

