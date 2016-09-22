// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gcbench

const ptrSize = 4 << (^uintptr(0) >> 63)

const frameSize = 512

// WithStack grows the stack by ~size bytes and calls f.
//
// The stack is grown with a mix of pointer and scalar data to
// simulate a real stack (though all pointers are nil).
func WithStack(size Bytes, f func()) {
	if size < frameSize {
		f()
	} else {
		withStack1(size, f)
	}
}

func withStack1(size Bytes, f func()) uintptr {
	// Use frameSize bytes of stack frame.
	var thing [(frameSize - 4*ptrSize) / ptrSize / 2]struct {
		s uintptr
		p *byte
	}
	if size <= frameSize {
		f()
	} else {
		withStack1(size-frameSize, f)
	}
	return thing[0].s
}
