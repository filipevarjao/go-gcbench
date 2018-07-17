// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Test STW latency with large regions of dirty stack.
package main

import (
	"flag"
	"os"
	"time"
	"fmt"
	"runtime"

	"github.com/aclements/go-gcbench/gcbench"
	"github.com/aclements/go-gcbench/gcbench/heapgen"
)

const (
	// The ballast has to be reasonably large (and have pointers)
	// so concurrent mark takes more than stackHighTime.
	ballastSize   = 128 << 20
	garbagePerSec = 1000 << 20

	stackHighTime = 10 * time.Millisecond
)

var ballast interface{}

func stack(id int) {
	for {
		gcbench.WithStack(*flagDirtyStack, func() {
			time.Sleep(stackHighTime)
		})
	}
}

var (
	flagDuration   = flag.Duration("benchtime", 10*time.Second, "steady state duration")
	flagGs         = flag.Int("gs", 10000, "start `n` goroutines")
	flagDirtyStack = gcbench.FlagBytes("dirty-stack", 10*gcbench.KB, "dirty approximately `bytes` of stack per goroutine each GC cycle")
)

func main() {
	memstats := new(runtime.MemStats)
	start := time.Now()
	flag.Parse()
	if flag.NArg() > 0 {
		flag.Usage()
		os.Exit(2)
	}

	gcbench.NewBenchmark("DirtyStack", benchMain).Config("gs", *flagGs).Config("dirty-stack", *flagDirtyStack).Run()
	elapsed := time.Since(start)
	fmt.Print("time: ", elapsed)
	printMemStats(memstats)
}

func benchMain() {
	m := heapgen.Measure(heapgen.MakeAST)
	ballast = heapgen.Generate(m.Gen, m.BytesRetained, ballastSize)

	for i := 0; i < *flagGs; i++ {
		go stack(i)
	}

	(&gcbench.Churner{
		BytesPerSec: garbagePerSec,
	}).Start()

	time.Sleep(*flagDuration)
	os.Exit(0)
}

func printMemStats(memstats *runtime.MemStats) {
   runtime.ReadMemStats(memstats)
   fmt.Print(" | TotalAlloc ", memstats.TotalAlloc)
   fmt.Print(" | mallocs ", memstats.Mallocs)
   fmt.Print(" | frees ", memstats.Mallocs - memstats.Frees)
   fmt.Println(" | GC cycles ", memstats.NumGC)
}

