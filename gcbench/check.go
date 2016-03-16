// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gcbench

import (
	"fmt"
	"os"
	"runtime"
)

type GCChecker struct {
	mstats runtime.MemStats
}

func NewGCChecker() *GCChecker {
	c := new(GCChecker)
	runtime.ReadMemStats(&c.mstats)
	return c
}

func (c *GCChecker) NumGCAtLeast(n int) {
	var mstats runtime.MemStats
	runtime.ReadMemStats(&mstats)
	if count := int(mstats.NumGC - c.mstats.NumGC); count < n {
		fmt.Fprintf(os.Stderr, "wanted NumGC >= %d, got %d", n, count)
		os.Exit(1)
	}
}
