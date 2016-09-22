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

func (d *LatencyDist) bounds() (minb, maxb int, any bool) {
	minb, maxb = -1, 1
	for i, count := range d.Buckets {
		if count > 0 {
			if minb == -1 {
				minb = i
			}
			maxb = i + 1
		}
	}
	any = minb != -1
	return
}

func (d *LatencyDist) Fprint(w io.Writer) {
	minb, maxb, any := d.bounds()
	if !any {
		fmt.Fprintf(w, "no samples\n")
		return
	}
	for b := minb; b < maxb; b++ {
		lo, hi := d.FromBucket(b)
		fmt.Fprintf(w, "[%12s,%12s) %d\n", lo, hi, d.Buckets[b])
	}
}

// FprintHist renders d to w as an ASCII art histogram.
//
// The body of the plot will fit within a width x height cell box.
// Ticks and tick labels will be placed outside that box.
func (d *LatencyDist) FprintHist(w io.Writer, width, height int) {
	minb, maxb, any := d.bounds()
	if !any {
		fmt.Fprintf(w, "no samples\n")
		return
	}

	// Compute plot column counts by assuming each distribution
	// bucket is uniform and resampling into columns.
	cols := make([]int64, width)
	colWidth := float64(maxb-minb) / float64(width)
	for b := minb; b < maxb; b++ {
		count := d.Buckets[b]
		left, right := float64(b-minb)/colWidth, float64(b-minb+1)/colWidth
		if right > float64(width) {
			// Numerical error can push right just over
			// width, which breaks loops below. Fix this.
			right = float64(width)
		}

		// If this column is strictly in bucket, take a fast path.
		lefti, _ := math.Modf(left)
		righti, _ := math.Modf(right)
		if lefti == righti {
			cols[int(lefti)] += count
			continue
		}

		// Distribute this bucket between left and right. To
		// avoid cumulative error, we think of this as a line
		// going from (left, 0) to (right, count). Each bucket
		// contains the difference of the line between its two
		// edges.
		partialCount := func(x float64) int64 {
			if x < left {
				return 0
			} else if x >= right {
				return count
			}
			return int64(float64(count) / (right - left) * (x - left))
		}
		for i := int(left); i < int(math.Ceil(right)); i++ {
			c1 := partialCount(float64(i))
			c2 := partialCount(float64(i + 1))
			cols[i] += c2 - c1
		}
	}

	// Get max column value.
	maxCount := int64(0)
	for _, count := range cols {
		if count > maxCount {
			maxCount = count
		}
	}

	// Render histogram body.
	fills := []rune(" ▁▂▃▄▅▆▇█")
	cells := make([][]rune, height+2)
	for i := range cells {
		cells[i] = make([]rune, width)
		for j := range cells[i] {
			cells[i][j] = fills[0]
		}
	}
	maxBar := float64(height) - 0.5
	for col, count := range cols {
		if count == 0 {
			continue
		}
		frac := maxBar * math.Log(float64(count)) / math.Log(float64(maxCount))
		for row := 0; row < height; row++ {
			filled := int((frac - float64(row)) * float64(len(fills)))
			if row == 0 && count > 0 && filled <= 0 {
				// Ensure we show something for
				// non-empty buckets.
				filled = 1
			} else if filled < 0 {
				filled = 0
			} else if filled >= len(fills) {
				filled = len(fills) - 1
			}
			cells[height-row-1][col] = fills[filled]
		}
	}

	// Render X ticks. Start with the first power of xBase >= minb.
	const xBase = 10
	mint, _ := d.FromBucket(minb)
	tick := time.Duration(math.Pow(xBase, math.Ceil(math.Log(float64(mint))/math.Log(xBase))))
	tickRow, labelRow := &cells[height], &cells[height+1]
	for {
		col := int(float64(d.ToBucket(tick)-minb) / colWidth)
		if col >= width {
			break
		}

		(*tickRow)[col] = '╵'
		label := []rune(tick.String())
		start := col - len(label)/2
		n := copy((*labelRow)[start:], label)
		if n < len(label) {
			// Extend the row to fit the label.
			*labelRow = append(*labelRow, label[n:]...)
		}
		tick *= xBase
	}

	// Render Y ticks.
	for row := 0; row < height; row++ {
		// Compute the value at mid-row. This is the inverse
		// of the "frac" calculation above.
		frac := 0.5 + float64(row)
		count := math.Exp(frac * math.Log(float64(maxCount)) / maxBar)
		label := fmt.Sprintf("╴%d", int(count+0.5))
		cells[height-row-1] = append(cells[height-row-1], []rune(label)...)
	}

	// Print results.
	for _, row := range cells {
		fmt.Fprint(w, string(row), "\n")
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
