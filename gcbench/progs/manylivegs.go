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
)

const (
	ballastSize   = 100 << 20
	garbageSize   = 10 << 20
	garbagePeriod = 100 * time.Millisecond

	stackPeriod = 50 * time.Millisecond
	liveSize    = 1 << 10
)

var (
	ballast []byte
	garbage []byte
)

func churn() {
	ballast = make([]byte, ballastSize)

	for {
		time.Sleep(garbagePeriod)
		garbage = make([]byte, garbageSize)
	}
}

func live(ch chan struct{}) {
	var s [liveSize]byte
	for {
		<-ch
		// read/write the stack, so it doesn't go unused
		for i := 0; i < len(s); i += 256 {
			s[i]++
		}
	}
}

var (
	flagDuration = flag.Duration("benchtime", 10*time.Second, "steady state duration")
	// 5e5 Gs uses about 2.5GB of memory.
	flagGs = flag.Int("gs", 5e5, "start `n` idle goroutines")
)

func main() {
	flag.Parse()
	if flag.NArg() > 0 {
		flag.Usage()
		os.Exit(2)
	}

	gcbench.NewBenchmark("LiveGs", benchMain).Config("gs", *flagGs).Run()
}

func benchMain() {
	var chs []chan struct{}

	for i := 0; i < *flagGs; i++ {
		ch := make(chan struct{})
		chs = append(chs, ch)
		go live(ch)
	}

	go churn()

	time.AfterFunc(10*time.Second, func() { os.Exit(0) })

	for range time.Tick(stackPeriod) {
		for _, ch := range chs {
			// TODO: Report jitter here. In
			// gc-pause-time-alt-2 I introduced some nasty
			// delay on this.
			ch <- struct{}{}
		}
	}
}
