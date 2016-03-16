// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Test STW latency and on large BSSes.
package main

import (
	"flag"
	"os"
	"time"

	"github.com/aclements/go-gcbench/gcbench"
)

const ptrSize = 4 << (^uintptr(0) >> 63)

const bssBytes = 1 << 30

// Pessimal case:
var thing [bssBytes / (2 * ptrSize)]struct {
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
	gcbench.NewBenchmark("LargeBSS", benchMain).Config("mb", bssBytes>>20).Run()
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
