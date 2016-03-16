// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Test STW latency and mark throughput on large, heterogeneous heaps.
package main

import (
	"flag"
	"os"
	"time"

	"github.com/aclements/go-gcbench/gcbench"
	"github.com/aclements/go-gcbench/gcbench/heapgen"
)

// Retain ~1 GB of heap. Let the heap grow to ~2 GB.
const goalBytes = 1 << 30

var sink1, sink2 interface{}

var (
	flagDuration = flag.Duration("benchtime", 20*time.Second, "steady state duration")
	flagMB       = flag.Int("mb", 1<<10, "retain `x` MB of heap")
)

func main() {
	flag.Parse()
	if flag.NArg() > 0 {
		flag.Usage()
		os.Exit(2)
	}

	gcbench.NewBenchmark("LargeHeap", benchMain).Config("mb", *flagMB).Config("heap", "AST").Run()
}

func benchMain() {
	m := heapgen.Measure(heapgen.MakeAST)
	println(m.BytesRetained, "bytes per AST")
	sink1 = heapgen.Generate(m.Gen, m.BytesRetained, *flagMB<<20)

	// On my laptop for 1.5 and 1.6, this takes another ~10
	// seconds to reach steady state.
	time.AfterFunc(*flagDuration, func() { os.Exit(0) })
	for {
		sink2 = m.Gen()
	}
}
