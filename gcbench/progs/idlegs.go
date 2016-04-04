// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Test STW latency with large numbers of idle goroutines.
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

var (
	flagDuration = flag.Duration("benchtime", 10*time.Second, "steady state duration")
	// 5e5 Gs uses about 1.5GB of memory.
	flagGs = flag.Int("idle-gs", 5e5, "start `n` idle goroutines")
)

func main() {
	flag.Parse()
	if flag.NArg() > 0 {
		flag.Usage()
		os.Exit(2)
	}

	gcbench.NewBenchmark("IdleGs", benchMain).Config("gs", *flagGs).Run()
}

func benchMain() {
	for i := 0; i < *flagGs; i++ {
		go func() { select {} }()
	}

	go churn()

	time.AfterFunc(*flagDuration, func() { os.Exit(0) })
	select {}
}
