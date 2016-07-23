// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gcbench

import (
	"bytes"
	"flag"
	"fmt"
	"math"
	"os"
	"os/exec"
	"runtime"
	"runtime/trace"
	"sort"
	"strconv"
	"strings"
	"time"
)

// TODO: Having the benchmarks all os.Exit to finish is a pain for
// things like metrics reporting and tracing.

var flagTrace = flag.String("trace", "", "write execution trace to `file`")

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
	// TODO: Have a flag or some easy way to print the GC trace of
	// the running benchmark.

	if gcbench := os.Getenv("GCBENCH"); gcbench != "" {
		if gcbench == b.FullName() {
			func() {
				if *flagTrace != "" {
					f, err := os.Create(*flagTrace)
					if err != nil {
						fmt.Fprintf(os.Stderr, "error creating log file: %v\n", err)
						os.Exit(1)
					}
					trace.Start(f)
					defer trace.Stop()
				}
				b.main()
			}()
		}
		os.Exit(0)
	}

	fmt.Printf("%s\t", b.FullName())

	godebug := os.Getenv("GODEBUG")
	if godebug != "" {
		godebug += ","
	}
	godebug += "gctrace=1"

	cmd := exec.Command(os.Args[0], os.Args[1:]...)
	cmd.Env = append([]string{"GODEBUG=" + godebug, "GCBENCH=" + b.FullName()}, os.Environ()...)
	startTime := time.Now()
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to run %s: %s\n%s\n", os.Args[0], err, indent(string(out)))
		return
	}
	endTime := time.Now()

	// Parse extra metrics.
	extra := map[string]float64{}
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		if !strings.HasPrefix(line, "metric ") {
			continue
		}
		fs := strings.Fields(line)
		if len(fs) != 3 {
			continue
		}
		v, err := strconv.ParseFloat(fs[1], 64)
		if err != nil {
			continue
		}
		extra[fs[2]] = v
	}
	extraKeys := []string{}
	for k := range extra {
		extraKeys = append(extraKeys, k)
	}
	sort.Strings(extraKeys)

	// Parse the GC trace.
	gctrace, err := ParseGCTrace(string(out))
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse output of %s: %s\n%s\n", os.Args[0], err, indent(string(out)))
		return
	}

	run := RunInfo{Trace: gctrace, StartTime: startTime, EndTime: endTime}

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
		vals[i] = metric.Fn(run)
		if math.IsNaN(vals[i]) {
			continue
		}
		fmt.Printf("%s%10s %s", align, sigfigs(vals[i]), metric.Label)
	}
	for _, k := range extraKeys {
		fmt.Printf("%s%10s %s", align, sigfigs(extra[k]), k)
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
		if !strings.HasPrefix(line, "gc ") && !strings.HasPrefix(line, "metric ") {
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

// ReportExtra can be used by a benchmark main function to report
// extra metrics.
func ReportExtra(metric string, val float64) {
	fmt.Fprintf(os.Stderr, "metric %v %s\n", val, metric)
}
