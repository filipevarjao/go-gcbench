// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package bench reads Go benchmarks results files.
//
// This format is specified at:
// https://github.com/golang/proposal/blob/master/design/14313-benchmark-format.md
package bench

import (
	"bufio"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Benchmark records the configuration and results of a single
// benchmark run (a single line of a benchmark results file).
type Benchmark struct {
	// Name is the name of the benchmark, without the "Benchmark"
	// prefix and without the trailing GOMAXPROCS number.
	Name string

	// Iterations is the number of times this benchmark executed.
	Iterations int

	// Config is the set of configuration pairs for this
	// Benchmark. These can be specified in both configuration
	// blocks and in individual benchmark lines. If the benchmark
	// name is of the form "BenchmarkX-N", the N is stripped out
	// and stored as "gomaxprocs" here.
	Config map[string]*Config

	// Result is the set of (unit, value) metrics for this
	// benchmark run.
	Result map[string]float64
}

// Config represents a single key/value configuration pair.
type Config struct {
	// Value is the parsed value of this configuration value.
	Value interface{}

	// RawValue is the value of this configuration value, exactly
	// as written in the original benchmark file.
	RawValue string

	// InBlock indicates that this configuration value was
	// specified in a configuration block line. Otherwise, it was
	// specified in the benchmark line.
	InBlock bool
}

var configRe = regexp.MustCompile(`^(\p{Ll}[^\p{Lu}\s\x85\xa0\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}]*):(?:[ \t]+(.*))?$`)

// Parse parses a standard Go benchmark results file from r. It
// returns a *Benchmark for each benchmark result line in the file.
// There may be many result lines for the same benchmark name and
// configuration, indicating that the benchmark was run multiple
// times.
//
// In the returned Benchmarks, all configuration values are strings.
// Use ParseValues to convert them to more structured types.
func Parse(r io.Reader) ([]*Benchmark, error) {
	benchmarks := []*Benchmark{}
	config := make(map[string]*Config)

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()

		if line == "testing: warning: no tests to run" {
			continue
		}

		// Configuration lines.
		m := configRe.FindStringSubmatch(line)
		if m != nil {
			config[m[1]] = &Config{Value: m[2], RawValue: m[2], InBlock: true}
			continue
		}

		// Benchmark lines.
		if strings.HasPrefix(line, "Benchmark") {
			b := parseBenchmark(line, config)
			if b != nil {
				benchmarks = append(benchmarks, b)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return benchmarks, nil
}

func parseBenchmark(line string, gconfig map[string]*Config) *Benchmark {
	// TODO: Consider using scanner to avoid the slice allocation.
	f := strings.Fields(line)
	if len(f) < 4 {
		return nil
	}
	if f[0] != "Benchmark" {
		next, _ := utf8.DecodeRuneInString(f[0][len("Benchmark"):])
		if !unicode.IsUpper(next) {
			return nil
		}
	}

	b := &Benchmark{
		Config: make(map[string]*Config),
		Result: make(map[string]float64),
	}

	// Copy global config.
	for k, v := range gconfig {
		b.Config[k] = v
	}

	// Parse name and configuration.
	name := strings.TrimPrefix(f[0], "Benchmark")
	if strings.Contains(name, "/") {
		parts := strings.Split(name, "/")
		b.Name = parts[0]
		for _, part := range parts[1:] {
			if i := strings.Index(part, ":"); i >= 0 {
				k, v := part[:i], part[i+1:]
				b.Config[k] = &Config{Value: v, RawValue: v}
			}
		}
	} else if i := strings.LastIndex(name, "-"); i >= 0 {
		_, err := strconv.Atoi(name[i+1:])
		if err == nil {
			b.Name = name[:i]
			b.Config["gomaxprocs"] = &Config{Value: name[i+1:], RawValue: name[i+1:]}
		} else {
			b.Name = name
		}
	} else {
		b.Name = name
	}

	// Parse iterations.
	n, err := strconv.Atoi(f[1])
	if err != nil || n <= 0 {
		return nil
	}
	b.Iterations = n

	// Parse results.
	for i := 2; i+2 <= len(f); i += 2 {
		val, err := strconv.ParseFloat(f[i], 64)
		if err != nil {
			continue
		}
		b.Result[f[i+1]] = val
	}

	return b
}

// ValueParser is a function that parses a string value into a
// structured type or returns an error if the string cannot be parsed.
type ValueParser func(string) (interface{}, error)

// DefaultValueParsers is the default sequence of value parsers used
// by ParseValues if no parsers are specified.
var DefaultValueParsers = []ValueParser{
	func(s string) (interface{}, error) { return strconv.Atoi(s) },
	func(s string) (interface{}, error) { return strconv.ParseFloat(s, 64) },
	func(s string) (interface{}, error) { return time.ParseDuration(s) },
}

// TODO: If ParseValues was part of Parse, we could avoid repeatedly
// parsing block configuration values.

// ParseValues parses the raw configuration values in benchmarks into
// structured types using best-effort pattern-based parsing.
//
// If all of the raw values for a given configuration key can be
// parsed by one of the valueParsers, ParseValues sets the parsed
// values to the results of that ValueParser. If multiple ValueParsers
// can parse all of the raw values, it uses the earliest such parser
// in the valueParsers list.
//
// If valueParsers is nil, it uses DefaultValueParsers.
func ParseValues(benchmarks []*Benchmark, valueParsers []ValueParser) {
	if valueParsers == nil {
		valueParsers = DefaultValueParsers
	}

	// Collect all configuration keys.
	keys := map[string]bool{}
	for _, b := range benchmarks {
		for k := range b.Config {
			keys[k] = true
		}
	}

	// For each configuration key, try value parsers in priority order.
	values := make([]interface{}, len(benchmarks))
	for key := range keys {
	tryParsers:
		for _, vp := range valueParsers {
			good := true
		tryValues:
			for i, b := range benchmarks {
				c, ok := b.Config[key]
				if !ok {
					continue
				}

				res, err := vp(c.RawValue)
				if err != nil {
					// Parse error. Fail this parser.
					good = false
					break tryValues
				}

				values[i] = res
			}

			if good {
				// This ValueParser converted all of
				// the values.
				for i, b := range benchmarks {
					if _, ok := b.Config[key]; ok {
						b.Config[key].Value = values[i]
					}
				}
				break tryParsers
			}
		}
	}
}
