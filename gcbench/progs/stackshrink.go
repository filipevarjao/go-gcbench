// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Test STW latency when a large number of stacks need to be shrunk.
//
// See golang.org/issue/12967
package main

import (
	"flag"
	"os"
	"runtime"
	"sync"
	"time"
	"fmt"

	"github.com/aclements/go-gcbench/gcbench"
)

const (
	ballastSize   = 10 << 20
	garbagePerSec = 10 << 20

	stackPeriod = 5 * time.Second
)

func stack(phase, a, b *sync.WaitGroup) {
	gcbench.WithStack(*flagLow, func() {
		for {
			gcbench.WithStack(*flagHigh-*flagLow, func() {
				phase.Done()
				a.Wait()
			})
			phase.Done()
			b.Wait()
		}
	})
}

var (
	flagDuration = flag.Duration("benchtime", 20*time.Second, "steady state duration")
	flagGs       = flag.Int("gs", 10000, "start `n` goroutines")
	flagLow      = gcbench.FlagBytes("low", 0, "retain approximately `bytes` of stack")
	flagHigh     = gcbench.FlagBytes("high", 10*gcbench.KB, "grow to approximately `bytes` of stack")
)

func main() {
	memstats := new(runtime.MemStats)
	start := time.Now()

	flag.Parse()
	if flag.NArg() > 0 {
		flag.Usage()
		os.Exit(2)
	}

	gcbench.NewBenchmark("StackShrink", benchMain).Config("gs", *flagGs).Config("low", *flagLow).Config("high", *flagHigh).Run()
	elapsed := time.Since(start)
	fmt.Print("time: ", elapsed)
	printMemStats(memstats)

}

func benchMain() {
	var churn = gcbench.Churner{
		BallastBytes: ballastSize,
		BytesPerSec:  garbagePerSec,
	}

	var phase, a, b sync.WaitGroup
	phase.Add(*flagGs)
	a.Add(1) // Grow stacks
	for i := 0; i < *flagGs; i++ {
		go stack(&phase, &a, &b)
	}
	// Wait for all stacks to be big.
	phase.Wait()

	time.AfterFunc(*flagDuration, func() { os.Exit(0) })

	for {
		// Shrink all stacks.
		phase.Add(*flagGs)
		b.Add(1)
		a.Add(-1)
		phase.Wait()

		// Let GC happen.
		var mstats0, mstats1 runtime.MemStats
		runtime.ReadMemStats(&mstats0)
		churn.Start()
		for {
			time.Sleep(10 * time.Millisecond)
			runtime.ReadMemStats(&mstats1)
			if mstats1.NumGC >= mstats0.NumGC+2 {
				break
			}
		}
		churn.Stop()

		// Grow all stacks.
		phase.Add(*flagGs)
		a.Add(1)
		b.Add(-1)
		phase.Wait()
	}
}

func printMemStats(memstats *runtime.MemStats) {
   runtime.ReadMemStats(memstats)
   fmt.Print(" | TotalAlloc ", memstats.TotalAlloc)
   fmt.Print(" | mallocs ", memstats.Mallocs)
   fmt.Print(" | frees ", memstats.Mallocs - memstats.Frees)
   fmt.Println(" | GC cycles ", memstats.NumGC)
}

