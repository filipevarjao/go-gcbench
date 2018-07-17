// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package heapgen

import (
	"runtime"
//aqui	"runtime/debug"
	"sync"
)

type Measurement struct {
	// Gen is the garbage generator function used for this
	// Measurement.
	Gen func() interface{}

	// BytesRetained is the bytes of heap retained by the result
	// of Gen().
	BytesRetained int

	// ObjectsRetained is the number of objects retained by the
	// result of Gen().
	ObjectsRetained int

	// BytesGarbage is the bytes of non-retained garbage produced
	// by calling Gen().
	BytesGarbage int
}

var sink interface{}

// Measure measures gen's effect on the heap. The system must be
// otherwise idle.
func Measure(gen func() interface{}) Measurement {
	// Warm up the generator.
	gen()

	// Clear the sink and GC everything.
	sink = nil
	runtime.GC()
	runtime.GC()

	// Measure total heap generated.
	var mstats0 runtime.MemStats
	runtime.ReadMemStats(&mstats0)
//aqui	gogc := debug.SetGCPercent(-1)
	sink = gen()
	var mstats1 runtime.MemStats
	runtime.ReadMemStats(&mstats1)
//aqui	debug.SetGCPercent(gogc)
	// GC and measure retained heap.
	runtime.GC()
	runtime.GC()
	var mstats2 runtime.MemStats
	runtime.ReadMemStats(&mstats2)

	return Measurement{
		Gen:             gen,
		BytesRetained:   int(mstats2.HeapInuse - mstats0.HeapInuse),
		ObjectsRetained: int(mstats2.HeapObjects - mstats0.HeapObjects),
		BytesGarbage:    int(mstats1.HeapInuse - mstats2.HeapInuse),
	}
}

// Generate generates garbage by running gen() bytesGoal/bytes1 times.
// It returns an object that retains all objects returned by gen.
func Generate(gen func() interface{}, bytes1, bytesGoal int) interface{} {
	count := (bytesGoal + bytes1 - 1) / bytes1
	out := make([]interface{}, count)
	procs := runtime.GOMAXPROCS(-1)
	var wg sync.WaitGroup
	wg.Add(procs)
	for p := 0; p < procs; p++ {
		go func(p int) {
			for i := p; i < len(out); i += procs {
				out[i] = gen()
			}
			wg.Done()
		}(p)
	}
	wg.Wait()
	return out
}

