// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gcbench

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

type Benchmark struct {
	name string
	cfg  []config
	main func()
}

type config struct {
	k, v string
}

func NewBenchmark(name string, main func()) *Benchmark {
	return &Benchmark{name, nil, main}
}

func (b *Benchmark) Config(name string, value interface{}) *Benchmark {
	b.cfg = append(b.cfg, config{name, fmt.Sprint(value)})
	return b
}

func (b *Benchmark) FullName() string {
	buf := bytes.NewBufferString("Benchmark" + b.name)
	cpus := runtime.GOMAXPROCS(-1)
	if len(b.cfg) > 0 {
		for _, c := range b.cfg {
			fmt.Fprintf(buf, "/%s:%s", c.k, c.v)
		}
		if cpus != 1 {
			fmt.Fprintf(buf, "/gomaxprocs:%d", cpus)
		}
	} else if cpus != 1 {
		fmt.Fprintf(buf, "-%d", cpus)
	}
	return buf.String()
}

func (b *Benchmark) Run() {
	if gcbench := os.Getenv("GCBENCH"); gcbench != "" {
		if gcbench == b.FullName() {
			b.main()
		}
		os.Exit(0)
	}

	fmt.Printf("%s\t", b.FullName())

	cmd := exec.Command(os.Args[0], os.Args[1:]...)
	cmd.Env = append([]string{"GODEBUG=gctrace=1", "GCBENCH=" + b.FullName()}, os.Environ()...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to run %s: %s\n%s\n", os.Args[0], err, indent(string(out)))
		return
	}

	// Parse the GC trace.
	gctrace, err := ParseGCTrace(string(out))
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse output of %s: %s\n%s\n", os.Args[0], err, indent(string(out)))
		return
	}

	// Print metrics.
	fmt.Printf("%d", 1)
	// The \t's are a horrible hack to keep everything technically
	// on one line, but to make that line readable in a terminal
	// by putting each result on a separate visual line.
	align := "\t"
	if os.Getenv("TERM") != "dumb" {
		align = strings.Repeat("\t", 15) + " "
	}
	vals := make([]float64, len(metrics))
	for i, metric := range metrics {
		vals[i] = metric.Fn(gctrace)
		if math.IsNaN(vals[i]) {
			continue
		}
		fmt.Printf("%s%10s %s", align, sigfigs(vals[i]), metric.Label)
	}
	fmt.Printf("\n")

	// Print warnings.
	for i, metric := range metrics {
		if metric.Check != nil && !math.IsNaN(vals[i]) {
			metric.Check(metric.Label, vals[i])
		}
	}

	// Print any non-GC output.
	nongc := []string{}
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		if !strings.HasPrefix(line, "gc ") {
			nongc = append(nongc, line)
		}
	}
	if len(nongc) > 0 {
		fmt.Fprintf(os.Stderr, "%s\n", strings.Join(nongc, "\n"))
	}
}

func indent(s string) string {
	return "    " + strings.Replace(s, "\n", "\n    ", -1)
}

// sigfigs formats v with three or more sigfigs.
func sigfigs(v float64) string {
	prec, m := 0, v
	for {
		if 99.5 <= m || m <= -99.5 || m*10 == m || m != m {
			return strconv.FormatFloat(v, 'f', prec, 64)
		}
		m *= 10
		prec++
	}
}
