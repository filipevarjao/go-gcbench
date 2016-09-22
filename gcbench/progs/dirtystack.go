// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Test STW latency with large regions of dirty stack.
package main

import (
	"flag"
	"os"
	"time"

	"github.com/aclements/go-gcbench/gcbench"
	"github.com/aclements/go-gcbench/gcbench/heapgen"
)

const (
	ptrSize = 4 << (^uintptr(0) >> 63)

	// The ballast has to be reasonably large (and have pointers)
	// so concurrent mark takes more than stackHighTime.
	ballastSize   = 128 << 20
	garbageSize   = 1 << 20
	garbagePeriod = 1 * time.Millisecond

	stackHighTime = 10 * time.Millisecond
)

var (
	ballast interface{}
	garbage []byte
)

func churn() {
	for {
		time.Sleep(garbagePeriod)
		garbage = make([]byte, garbageSize)
	}
}

func stack(id int) {
	for {
		withStack(*flagDirtyStack, func() {
			time.Sleep(stackHighTime)
		})
	}
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
	flagDuration   = flag.Duration("benchtime", 10*time.Second, "steady state duration")
	flagGs         = flag.Int("gs", 10000, "start `n` goroutines")
	flagDirtyStack = gcbench.FlagBytes("dirty-stack", 10*gcbench.KB, "dirty approximately `bytes` of stack per goroutine each GC cycle")
)

func main() {
	flag.Parse()
	if flag.NArg() > 0 {
		flag.Usage()
		os.Exit(2)
	}

	gcbench.NewBenchmark("DirtyStack", benchMain).Config("gs", *flagGs).Config("dirty-stack", *flagDirtyStack).Run()
}

func benchMain() {
	m := heapgen.Measure(heapgen.MakeAST)
	ballast = heapgen.Generate(m.Gen, m.BytesRetained, ballastSize)

	for i := 0; i < *flagGs; i++ {
		go stack(i)
	}
	go churn()

	time.Sleep(*flagDuration)
	os.Exit(0)
}
