// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gcbench

import (
	"fmt"
	"io"
	"math"
	"sync/atomic"
	"time"
)

// LatencyDist records a distribution of latencies in exponential
// buckets between 1 ns and 1 sec.
type LatencyDist struct {
	N       int64
	Max     time.Duration
	Buckets [256]int64
}

const latencyMax = time.Second

func (d *LatencyDist) Start() *LatencyTracker {
	return &LatencyTracker{dist: d, last: time.Now()}
}

func (d *LatencyDist) Add(t time.Duration) {
	atomic.AddInt64(&d.N, 1)

	max := time.Duration(atomic.LoadInt64((*int64)(&d.Max)))
	for t > max {
		if atomic.CompareAndSwapInt64((*int64)(&d.Max), int64(max), int64(t)) {
			break
		}
		max = time.Duration(atomic.LoadInt64((*int64)(&d.Max)))
	}

	b := d.ToBucket(t)
	atomic.AddInt64(&d.Buckets[b], 1)
}

var latencyBase = math.Log(float64(latencyMax))

func (d *LatencyDist) ToBucket(t time.Duration) int {
	if t < 1 {
		return 0
	}
	// Log base latencyMax.
	b := int(float64(len(d.Buckets)) * math.Log(float64(t)) / latencyBase)
	if b < 0 {
		return 0
	}
	if b >= len(d.Buckets) {
		return len(d.Buckets) - 1
	}
	return b
}

func (d *LatencyDist) FromBucket(b int) (lo, hi time.Duration) {
	// b = n * log_1s(t) => t = 1s ^ (b / n)
	if b == 0 {
		lo = 0
	} else {
		lo = time.Duration(math.Pow(float64(latencyMax), float64(b)/float64(len(d.Buckets))))
	}
	hi = time.Duration(math.Pow(float64(latencyMax), float64(b+1)/float64(len(d.Buckets))))
	return
}

func (d *LatencyDist) Fprint(w io.Writer) {
	minb, maxb := -1, 1
	for i, count := range d.Buckets {
		if count > 0 {
			if minb == -1 {
				minb = i
			}
			maxb = i
		}
	}
	if minb == -1 {
		fmt.Fprintf(w, "no samples\n")
		return
	}
	for b := minb; b <= maxb; b++ {
		lo, hi := d.FromBucket(b)
		fmt.Fprintf(w, "[%12s,%12s) %d\n", lo, hi, d.Buckets[b])
	}
}

func (d *LatencyDist) Quantile(q float64) time.Duration {
	// Find the bucket containing this quantile.
	n := int64(q * float64(d.N+1))
	if n < 0 {
		n = 0
	}
	if n > d.N-1 {
		n = d.N - 1
	}
	b := 0
	for ; n >= d.Buckets[b]; n, b = n-d.Buckets[b], b+1 {
	}

	// Take the midpoint of this bucket.
	// TODO: Assume samples are log distributed in bucket.
	lo, hi := d.FromBucket(b)
	mid := (lo + hi) / 2
	if mid > d.Max {
		return d.Max
	}
	return mid
}

type LatencyTracker struct {
	dist *LatencyDist
	last time.Time
}

func (t *LatencyTracker) Tick() {
	now := time.Now()
	t.dist.Add(now.Sub(t.last))
	t.last = now
}

func (t *LatencyTracker) Done() {
}
