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
	B  Bytes = 1
	KB       = 1 << 10
	MB       = 1 << 20
	GB       = 1 << 30
	TB       = 1 << 40
	PB       = 1 << 50
	EB       = 1 << 60
	ZB       = 1 << 70
	YB       = 1 << 80
)

func (b Bytes) String() string {
	for i, s := range si {
		if b%1024 != 0 || b == 0 || i == len(si)-1 {
			return fmt.Sprintf("%d%sB", b, s)
		}
		b /= 1024
	}
	panic("not reached")
}

func (b *Bytes) Set(s string) error {
	var num float64
	var unit string
	_, err := fmt.Sscanf(s, "%g%s", &num, &unit)
	if err == nil {
		for _, s := range si {
			if unit == s+"B" {
				*b = Bytes(num)
				return nil
			}
			num *= 1024
		}
	}
	return fmt.Errorf("expected <num><prefix>B")
}

func FlagBytes(name string, value Bytes, usage string) *Bytes {
	flag.Var(&value, name, usage)
	return &value
}
