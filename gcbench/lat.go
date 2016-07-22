// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gcbench

import (
	"sync/atomic"
	"time"
)

type LatencyDist struct {
	N   int64
	Max time.Duration
}

func (d *LatencyDist) Start() *LatencyTracker {
	return &LatencyTracker{dist: d, last: time.Now()}
}

func (d *LatencyDist) add(t time.Duration) {
	atomic.AddInt64(&d.N, 1)
	max := time.Duration(atomic.LoadInt64((*int64)(&d.Max)))
	for t > max {
		if atomic.CompareAndSwapInt64((*int64)(&d.Max), int64(max), int64(t)) {
			break
		}
		max = time.Duration(atomic.LoadInt64((*int64)(&d.Max)))
	}
}

type LatencyTracker struct {
	dist *LatencyDist
	last time.Time
}

func (t *LatencyTracker) Tick() {
	now := time.Now()
	t.dist.add(now.Sub(t.last))
	t.last = now
}

func (t *LatencyTracker) Done() {
}
