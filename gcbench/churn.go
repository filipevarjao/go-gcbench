// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gcbench

import "time"

// A Churner churns the heap to cause GCs at some rate.
type Churner struct {
	// BallastBytes is the minimum bytes to retain in the heap.
	//
	// This is implemented as a simple []byte allocation. A caller
	// may create their own ballast and set this to 0.
	BallastBytes uint64

	// BytesPerSec is how many bytes of garbage to allocate per
	// second.
	//
	// If BytesPerSec == BallastBytes, there will be approximately
	// one GC per second.
	BytesPerSec uint64

	ballast []byte

	ticker        *time.Ticker
	stop, stopped chan struct{}
}

var churnSink interface{}

func (c *Churner) Start() {
	const allocsPerSec = 10

	if c.stop != nil {
		panic("Churner already running")
	}
	stop, stopped := make(chan struct{}), make(chan struct{})
	c.stop, c.stopped = stop, stopped

	if c.ballast == nil && c.BallastBytes > 0 {
		c.ballast = make([]byte, c.BallastBytes)
	}
	if c.ticker == nil {
		c.ticker = time.NewTicker(time.Second / allocsPerSec)
	}

	go func() {
	loop:
		for {
			select {
			case <-stop:
				break loop
			case <-c.ticker.C:
			}
			churnSink = make([]byte, c.BytesPerSec/allocsPerSec)
		}
		close(stopped)
		// Keep the ballast alive.
		if c.ballast != nil {
			c.ballast[0] = 0
		}
	}()
}

func (c *Churner) Stop() {
	close(c.stop)
	<-c.stopped
	c.stop, c.stopped = nil, nil
}
