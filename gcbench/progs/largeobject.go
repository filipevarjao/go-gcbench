// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Test STW and goroutine latency with large heap objects.
//
// This is inspired by Rhys Hiltner's benchmark from issue #16293.
package main

import (
	"flag"
	"os"
	"runtime"
	"time"

	"github.com/aclements/go-gcbench/gcbench"
)

const (
	ptrSize = 4 << (^uintptr(0) >> 63)

	ballastBytes  = 128 << 20
	garbagePerSec = 128 << 20
)

var (
	uintptrs    []*uintptr
	ballast     interface{}
	workerSinks []interface{}
	lat         gcbench.LatencyDist
)

var (
	flagDuration = flag.Duration("benchtime", 10*time.Second, "steady state duration")
	flagObjBytes = gcbench.FlagBytes("obj-size", 32*gcbench.MB, "large object size")
)

func main() {
	flag.Parse()
	if flag.NArg() > 0 {
		flag.Usage()
		os.Exit(2)
	}

	gcbench.NewBenchmark("LargeObject", benchMain).Config("obj-size", *flagObjBytes).Run()
}

func benchMain() {
	// Allocate a lot of uintptr objects.
	uintptrs = make([]*uintptr, *flagObjBytes/ptrSize)
	for i := range uintptrs {
		uintptrs[i] = new(uintptr)
	}

	// Create the ballast. uintptrs already took 2*objectBytes bytes.
	b := make([][]*uintptr, (ballastBytes-2*(*flagObjBytes))/(*flagObjBytes))
	for i := range b {
		b[i] = makeBigObject()
	}

	// Run workers, which allocate to force GC and perform
	// assists.
	workerSinks = make([]interface{}, 2*runtime.GOMAXPROCS(-1))
	for i := range workerSinks {
		go worker(i)
	}

	time.Sleep(*flagDuration)

	lat.FprintHist(os.Stderr, 70, 5)
	gcbench.ReportExtra("P99-latency-ns", float64(lat.Quantile(0.99)))
	gcbench.ReportExtra("P99.9-latency-ns", float64(lat.Quantile(0.999)))
	gcbench.ReportExtra("max-latency-ns", float64(lat.Max))
}

func makeBigObject() []*uintptr {
	obj := make([]*uintptr, *flagObjBytes/ptrSize)
	copy(obj, uintptrs)
	return obj
}

func worker(id int) {
	const allocsPerSec = 100
	for {
		t := time.Now()
		workerSinks[id] = make([]byte, garbagePerSec/allocsPerSec/len(workerSinks))
		lat.Add(time.Since(t))
		time.Sleep(time.Second / allocsPerSec)
	}
}
