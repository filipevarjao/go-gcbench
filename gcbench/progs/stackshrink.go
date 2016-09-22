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

	"github.com/aclements/go-gcbench/gcbench"
)

const (
	ptrSize = 4 << (^uintptr(0) >> 63)

	ballastSize   = 10 << 20
	garbagePerSec = 10 << 20

	stackPeriod = 5 * time.Second
)

func stack(phase, a, b *sync.WaitGroup) {
	withStack(*flagLow, func() {
		for {
			withStack(*flagHigh-*flagLow, func() {
				phase.Done()
				a.Wait()
			})
			phase.Done()
			b.Wait()
		}
	})
}

const frameSize = 512

func withStack(size gcbench.Bytes, f func()) {
	if size < frameSize {
		f()
	} else {
		withStack1(size, f)
	}
}

func withStack1(size gcbench.Bytes, f func()) uintptr {
	// Use frameSize bytes of stack frame.
	var thing [(frameSize - 4*ptrSize) / ptrSize / 2]struct {
		s uintptr
		p *byte
	}
	if size <= frameSize {
		f()
	} else {
		withStack1(size-frameSize, f)
	}
	return thing[0].s
}

var (
	flagDuration = flag.Duration("benchtime", 20*time.Second, "steady state duration")
	flagGs       = flag.Int("gs", 10000, "start `n` goroutines")
	flagLow      = gcbench.FlagBytes("low", 0, "retain approximately `bytes` of stack")
	flagHigh     = gcbench.FlagBytes("high", 10*gcbench.KB, "grow to approximately `bytes` of stack")
)

func main() {
	flag.Parse()
	if flag.NArg() > 0 {
		flag.Usage()
		os.Exit(2)
	}

	gcbench.NewBenchmark("StackShrink", benchMain).Config("gs", *flagGs).Config("low", *flagLow).Config("high", *flagHigh).Run()
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
