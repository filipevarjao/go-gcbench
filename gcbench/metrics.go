// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gcbench

import (
	"fmt"
	"math"
	"os"
	"reflect"
	"sort"
	"time"
)

type RunInfo struct {
	Trace GCTrace

	StartTime, EndTime time.Time
}

type Metric struct {
	Label string
	Fn    func(RunInfo) float64
	Check func(name string, value float64)
}

var metrics = []Metric{
	{"GCs/op", gcsPerOp, warnIf("<", 5)},
	{"GCs/sec", gcsPerSec, nil},
	{"95%ile-ns/sweepTerm", distMetric(nsPerSweepTerm, 0.95), warnIf(">=", 5e6)},
	{"95%ile-ns/markTerm", distMetric(nsPerMarkTerm, 0.95), warnIf(">=", 5e6)},
	{"MB-marked/CPU/sec", markedMBPerCPUSec, nil},
	{"95%ile-heap-overshoot", distMetric(heapOvershoot, 0.95), warnIf(">", 0)},
	{"5%ile-heap-overshoot", distMetric(heapOvershoot, 0.05), warnIf("<", -.2)},
	{"95%ile-CPU-util", distMetric(cpuUtil, 0.95), warnIf(">", .5)},
}

func gcsPerOp(run RunInfo) float64 {
	t := run.Trace.WithoutForced()
	return float64(len(t))
}

func gcsPerSec(run RunInfo) float64 {
	t := run.Trace.WithoutForced()
	if len(t) == 0 {
		return 0
	}
	// Use the time between the first non-forced GC and the end of
	// execution.
	duration := run.EndTime.Sub(run.StartTime) - t[0].Start
	return float64(len(t)) / duration.Seconds()
}

func nsPerSweepTerm(run RunInfo) distribution {
	t := run.Trace.WithoutForced()
	return distribution(float64s(extract(t, "ClockSweepTerm").([]time.Duration)))
}

func nsPerMarkTerm(run RunInfo) distribution {
	t := run.Trace.WithoutForced()
	return distribution(float64s(extract(t, "ClockMarkTerm").([]time.Duration)))
}

func markedMBPerCPUSec(run RunInfo) float64 {
	t := run.Trace.WithoutForced()
	// Compute average overall rate.
	markTotal := sum(float64s(extract(t, "CPUMark").([]time.Duration)))
	markedTotal := sum(float64s(extract(t, "HeapMarked").([]Bytes)))
	return markedTotal * 1e9 / (markTotal * 1024 * 1024)
}

func heapOvershoot(run RunInfo) distribution {
	t := run.Trace.WithoutForced()
	var over distribution
	actual := extract(t, "HeapActual").([]Bytes)
	goal := extract(t, "HeapGoal").([]Bytes)
	for i := range actual {
		// Ignore very small heaps.
		if goal[i] < 10*MB {
			continue
		}
		over = append(over, float64(actual[i])/float64(goal[i])-1)
	}
	return over
}

func cpuUtil(run RunInfo) distribution {
	t := run.Trace.WithoutForced()
	var util distribution
	cpuAssist := extract(t, "CPUAssist").([]time.Duration)
	cpuBackground := extract(t, "CPUBackground").([]time.Duration)
	clockMark := extract(t, "ClockMark").([]time.Duration)
	procs := extract(t, "Procs").([]int)
	for i := range cpuAssist {
		if t[i].Format == Trace1_5 {
			// 1.5 had some accounting problem that causes
			// this to often go over 1.
			continue
		}
		if clockMark[i] != 0 {
			util = append(util, (float64(cpuAssist[i])+float64(cpuBackground[i]))/(float64(clockMark[i])*float64(procs[i])))
		}
	}
	return util
}

type distribution []float64

// distMetric transforms a distribution metric into a point metric at
// the specified percentile.
func distMetric(f func(RunInfo) distribution, pct float64) func(RunInfo) float64 {
	return func(run RunInfo) float64 {
		return pctile([]float64(f(run)), pct)
	}
}

// extract takes a slice []T where T is a struct and returns a slice
// of T.name.
func extract(slice interface{}, name string) interface{} {
	sv := reflect.ValueOf(slice)
	len := sv.Len()

	field, ok := sv.Type().Elem().FieldByName(name)
	if !ok {
		panic("unknown field: " + name)
	}
	var out reflect.Value
	if name == "Procs" {
		p := reflect.TypeOf(make([]int, len))
		out = reflect.MakeSlice(p, len, len)
	} else if name == "HeapMarked" || name == "HeapActual" || name == "HeapGoal" {
		p := reflect.TypeOf(make([]Bytes, len))
		out = reflect.MakeSlice(p, len, len)
	} else if name == "CPUAssist" || name == "ClockSweepTerm" || name == "ClockMarkTerm" || name == "CPUMark" || name == "CPUBackground" || name == "ClockMark"{
		p := reflect.TypeOf(make([]time.Duration, len))
		out = reflect.MakeSlice(p, len, len)
	}// else {
	//	fmt.Println("There is someone else ", name)
	//	out = reflect.MakeSlice(reflect.SliceOf(field.Type), len, len)
	//}
	for i := 0; i < len; i++ {
		out.Index(i).Set(sv.Index(i).FieldByIndex(field.Index))

	}
	return out.Interface()
}

var float64Type = reflect.TypeOf(float64(0.0))

// float64s converts a slice of integer or floating-point values to
// []float64.
func float64s(slice interface{}) []float64 {
	sv := reflect.ValueOf(slice)
	len := sv.Len()
	out := make([]float64, len)
	for i := 0; i < len; i++ {
//		if sv.Index(i).Type().Kind() == reflect.Int64 {
//			fmt.Println("Original ", sv.Index(i).Type().Name(), sv.Index(i).Int())
			
		//n := sv.Index(i).Int()
		out[i] = float64(sv.Index(i).Int())
	//	out[i] = sv.Index(i).Convert(float64Type).Float()
	}
	return out
}

func pctile(xs []float64, pct float64) float64 {
	sort.Float64s(xs)
	if len(xs) == 0 {
		return math.NaN()
	}
	return xs[int(float64(len(xs)-1)*pct)]
}

func sum(xs []float64) float64 {
	sum := 0.0
	for _, x := range xs {
		sum += x
	}
	return sum
}

// warnIf returns a metric check function that compares the metric
// value to the threshold using the given comparison and prints a
// warning if the comparison is true.
func warnIf(compare string, threshold float64) func(string, float64) {
	var fn func(a, b float64) bool
	switch compare {
	case ">":
		fn = func(a, b float64) bool { return a > b }
	case ">=":
		fn = func(a, b float64) bool { return a >= b }
	case "<=":
		fn = func(a, b float64) bool { return a <= b }
	case "<":
		fn = func(a, b float64) bool { return a < b }
	default:
		panic(fmt.Sprintf("unknown comparison operator %q", compare))
	}
	return func(name string, value float64) {
		if fn(value, threshold) {
			fmt.Fprintf(os.Stderr, "Warning: %s %s %s %s\n", sigfigs(value), name, compare, sigfigs(threshold))
		}
	}
}

