// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gcbench

import (
	"reflect"
	"sort"
	"time"
)

type Metric struct {
	Label string
	Fn    func(GCTrace) float64
}

var metrics = []Metric{
	{"GCs/sec", gcsPerSec},
	{"95%ile-ns/sweepTerm", nsPerSweepTerm95},
	{"95%ile-ns/markTerm", nsPerMarkTerm95},
	{"marked-MB/CPU/sec", markedMBPerCPUSec},
}

func gcsPerSec(t GCTrace) float64 {
	t = t.WithoutForced()
	if len(t) == 0 {
		return 0
	}
	return float64(len(t)) / t[len(t)-1].End.Seconds()
}

func nsPerSweepTerm95(t GCTrace) float64 {
	t = t.WithoutForced()
	return pctile(float64s(extract(t, "ClockSweepTerm").([]time.Duration)), .95)
}

func nsPerMarkTerm95(t GCTrace) float64 {
	t = t.WithoutForced()
	return pctile(float64s(extract(t, "ClockMarkTerm").([]time.Duration)), .95)
}

func markedMBPerCPUSec(t GCTrace) float64 {
	t = t.WithoutForced()
	markTotal := sum(float64s(extract(t, "CPUMark").([]time.Duration)))
	markedTotal := sum(float64s(extract(t, "HeapMarked").([]Bytes)))
	return markedTotal * 1e9 / (markTotal * 1024 * 1024)
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
	out := reflect.MakeSlice(reflect.SliceOf(field.Type), len, len)
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
		out[i] = sv.Index(i).Convert(float64Type).Float()
	}
	return out
}

func pctile(xs []float64, pct float64) float64 {
	sort.Float64s(xs)
	return xs[int(float64(len(xs)-1)*pct)]
}

func sum(xs []float64) float64 {
	sum := 0.0
	for _, x := range xs {
		sum += x
	}
	return sum
}
