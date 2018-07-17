// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gcbench

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type GCTrace []GCCycle

type GCCycle struct {
	// N is the 1-based index of this GC cycle.
	N int

	// Format indicates the variant of the GC trace format for this line.
	Format TraceFormat

	// Start and End are the times this GC cycle started and
	// ended, relative to when the program began executing.
	Start, End time.Duration

	// Util is the overall CPU utilized by GC since the program
	// began executing. Util will be in [0, 1].
	Util float64

	// Forced is true is this GC cycle is a forced STW cycle.
	Forced bool

	// Clock* are the wall-clock durations of each phase.
	// ClockSweepTerm, ClockSync, and ClockMarkTerm are STW.
	// On Go 1.6, ClockRootScan and ClockSync will be 0.
	ClockSweepTerm, ClockRootScan, ClockSync, ClockMark, ClockMarkTerm time.Duration

	// CPU* are the CPU times of each phase.
	CPUSweepTerm, CPURootScan, CPUSync, CPUMark, CPUMarkTerm time.Duration

	// CPUAssist, CPUBackground, and CPUIdle break down CPUMark in
	// to its components.
	CPUAssist, CPUBackground, CPUIdle time.Duration

	// HeapTrigger is the size of the heap at which this GC cycle
	// was triggered.
	HeapTrigger Bytes

	// HeapActual is the size of the heap when this GC cycle
	// finished (before sweeping).
	HeapActual Bytes

	// HeapMarked is the bytes of heap that this GC cycle marked
	// (which will be retained by sweeping).
	HeapMarked Bytes

	// HeapGoal is the size of the heap this GC cycle was aiming
	// to finish at.
	HeapGoal Bytes

	// Procs is the value of GOMAXPROCS during this GC cycle.
	Procs int
}

type TraceFormat int

const (
	// Trace1_5 is the trace format from Go 1.5.x.
	Trace1_5 TraceFormat = 1 + iota

	// Trace1_6 is the trace format from Go 1.6.x.
	Trace1_6
)

var (
	gcTraceLine  = regexp.MustCompile(`(?m)^gc #?([0-9]+) @([0-9.]+)s ([0-9]+)%: (.*)`)
	gcTraceClock = regexp.MustCompile(`^([+0-9.]+) ms clock$`)
	gcTraceCPU   = regexp.MustCompile(`^([+/0-9.]+) ms cpu$`)
	gcTraceHeap  = regexp.MustCompile(`^([0-9.]+)->([0-9.]+)->([0-9.]+) MB$`)
	gcTraceGoal  = regexp.MustCompile(`^([0-9]+) MB goal$`)
	gcTraceProcs = regexp.MustCompile(`^([0-9]+) P$`)
)

func ParseGCTrace(s string) (GCTrace, error) {
	lines := gcTraceLine.FindAllStringSubmatch(s, -1)
	out := make([]GCCycle, 0, len(lines))
	for _, line := range lines {
		c := GCCycle{
			N:     atoi(line[1]),
			Start: time.Duration(atof(line[2]) * 1e9),
			Util:  atof(line[3]) / 100,
		}

		if strings.HasSuffix(line[4], " (forced)") {
			c.Forced = true
			str := len(line[4])
			line[4] = line[4][:str-9] //strings.TrimSuffix(line[4], " (forced)")
		}

		// Process parts.
		for _, part := range strings.Split(line[4], ",") {
			part = strings.TrimSpace(part)

			m := gcTraceClock.FindStringSubmatch(part)
			if m != nil {
				var phases []time.Duration
				var sum time.Duration
				for _, p := range strings.Split(m[1], "+") {
					dur := msToDur(p)
					phases = append(phases, dur)
					sum += dur
				}
				c.End = c.Start + sum
				switch len(phases) {
				case 5: // Go 1.5
					c.Format = Trace1_5
					c.ClockSweepTerm = phases[0]
					c.ClockRootScan = phases[1]
					c.ClockSync = phases[2]
					c.ClockMark = phases[3]
					c.ClockMarkTerm = phases[4]
				case 3: // Go 1.6
					c.Format = Trace1_6
					c.ClockSweepTerm = phases[0]
					c.ClockMark = phases[1]
					c.ClockMarkTerm = phases[2]
				default:
					return nil, fmt.Errorf("unexpected number of phases: %d", len(phases))
				}
				continue
			}

			m = gcTraceCPU.FindStringSubmatch(part)
			if m != nil {
				var phases []time.Duration
				for _, p := range strings.Split(m[1], "+") {
					sub := strings.Split(p, "/")
					if len(sub) > 1 {
						c.CPUAssist = msToDur(sub[0])
						c.CPUBackground = msToDur(sub[1])
						c.CPUIdle = msToDur(sub[2])
					}
					var sum time.Duration
					for _, t := range sub {
						sum += msToDur(t)
					}
					phases = append(phases, sum)
				}
				switch len(phases) {
				case 5: // Go 1.5
					c.CPUSweepTerm = phases[0]
					c.CPURootScan = phases[1]
					c.CPUSync = phases[2]
					c.CPUMark = phases[3]
					c.CPUMarkTerm = phases[4]
				case 3: // Go 1.6
					c.CPUSweepTerm = phases[0]
					c.CPUMark = phases[1]
					c.CPUMarkTerm = phases[2]
				default:
					return nil, fmt.Errorf("unexpected number of phases: %d", len(phases))
				}
				continue
			}

			m = gcTraceHeap.FindStringSubmatch(part)
			if m != nil {
				c.HeapTrigger = mbToBytes(m[1])
				c.HeapActual = mbToBytes(m[2])
				c.HeapMarked = mbToBytes(m[3])
				continue
			}

			m = gcTraceGoal.FindStringSubmatch(part)
			if m != nil {
				c.HeapGoal = mbToBytes(m[1])
				continue
			}

			m = gcTraceProcs.FindStringSubmatch(part)
			if m != nil {
				c.Procs = atoi(m[1])
				continue
			}

			return nil, fmt.Errorf("failed to parse part of gctrace line: %q", part)
		}

		out = append(out, c)
	}

	return GCTrace(out), nil
}

func atoi(s string) int {
	i, err := strconv.Atoi(s)
	if err != nil {
		panic(err)
	}
	return i
}

func atof(s string) float64 {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		panic(err)
	}
	return f
}

func msToDur(s string) time.Duration {
	return time.Duration(atof(s) * 1e6)
}

func mbToBytes(s string) Bytes {
	return Bytes(atof(s) * (1024 * 1024))
}

func (t GCTrace) WithoutForced() GCTrace {
	out := make(GCTrace, 0)
	for _, c := range t {
		if !c.Forced {
			out = append(out, c)
		}
	}
	return out
}
