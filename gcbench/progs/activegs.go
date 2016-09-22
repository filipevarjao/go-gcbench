// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Test STW latency with large numbers of active goroutines.
//
// See golang.org/issue/14420
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
	// so concurrent mark takes more than stackPeriod.
	ballastSize   = 100 << 20
	garbagePerSec = 100 << 20

	stackPeriod = 10 * time.Millisecond
)

var ballast interface{}

func live(ch chan struct{}) {
	withStack(*flagStackSize, func() {
		var x byte
		for {
			<-ch
			x++
		}
	})
}

const frameSize = 512

func withStack(size gcbench.Bytes, f func()) {
	// TODO: Make this a gcbench package util?
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
	flagDuration = flag.Duration("benchtime", 10*time.Second, "steady state duration")
	// 5e5 Gs uses about 2.5GB of memory.
	flagGs        = flag.Int("active-gs", 5e5, "start `n` active goroutines")
	flagStackSize = gcbench.FlagBytes("stack-size", 1*gcbench.KB, "stack size")
)

func main() {
	flag.Parse()
	if flag.NArg() > 0 {
		flag.Usage()
		os.Exit(2)
	}

	gcbench.NewBenchmark("ActiveGs", benchMain).Config("active-gs", *flagGs).Config("stack-size", *flagStackSize).Run()
}

func benchMain() {
	m := heapgen.Measure(heapgen.MakeAST)
	ballast = heapgen.Generate(m.Gen, m.BytesRetained, ballastSize)

	var chs []chan struct{}

	for i := 0; i < *flagGs; i++ {
		ch := make(chan struct{})
		chs = append(chs, ch)
		go live(ch)
	}

	(&gcbench.Churner{
		BytesPerSec: garbagePerSec,
	}).Start()

	time.AfterFunc(*flagDuration, func() { os.Exit(0) })

	for range time.Tick(stackPeriod) {
		//begin := time.Now()
		for _, ch := range chs {
			// TODO: Report jitter here. In
			// gc-pause-time-alt-2 I introduced some nasty
			// delay on this.
			ch <- struct{}{}
		}
		//fmt.Println("loop took", time.Since(begin))
	}
}
