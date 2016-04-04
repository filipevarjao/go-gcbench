// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gcbench

import (
	"flag"
	"fmt"
)

type Bytes int

var si = []string{"", "k", "M", "G", "T", "P", "E", "Z", "Y"}

const (
	B Bytes = 1

	KB Bytes = 1e3
	MB Bytes = 1e6
	GB Bytes = 1e9
	TB Bytes = 1e12
	PB Bytes = 1e15
	EB Bytes = 1e18
	//ZB Bytes = 1e21
	//YB Bytes = 1e24

	KiB Bytes = 1 << 10
	MiB Bytes = 1 << 20
	GiB Bytes = 1 << 30
	TiB Bytes = 1 << 40
	PiB Bytes = 1 << 50
	EiB Bytes = 1 << 60
	//ZiB Bytes = 1 << 70
	//YiB Bytes = 1 << 80
)

func (b Bytes) String() string {
	f := float64(b)
	for i, s := range si {
		if f < 1000 || i == len(si)-1 {
			return fmt.Sprintf("%g%sB", f, s)
		}
		f /= 1000
	}
	panic("not reached")
}

func (b *Bytes) Set(s string) error {
	var num float64
	var unit string
	_, err := fmt.Sscanf(s, "%g%s", &num, &unit)
	if err == nil {
		// Try SI prefixes first.
		onum := num
		for _, s := range si {
			if unit == s+"B" {
				*b = Bytes(num)
				return nil
			}
			num *= 1000
		}
		// Try binary prefixes.
		num = onum
		for _, s := range si {
			if unit == s+"iB" {
				*b = Bytes(num)
				return nil
			}
			num *= 1024
		}
	}
	return fmt.Errorf("expected <num><SI or binary prefix>B")
}

func FlagBytes(name string, value Bytes, usage string) *Bytes {
	flag.Var(&value, name, usage)
	return &value
}

func ParseBytes(s string) (Bytes, error) {
	var b Bytes
	err := b.Set(s)
	return b, err
}
