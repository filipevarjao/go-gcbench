// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package heapgen

type node2 struct {
	ptrs [2]*node2
}

// MakeDeBruijn2 constructs a de Bruijn graph of objects of degree 2
// with exactly 2**power nodes. Each node in the graph has exactly 2
// out-edges and 2 in-edges and the diameter of the graph is power.
func MakeDeBruijn2(power int) interface{} {
	const degree = 2
	//numNodes := int(math.Pow(degree, float64(power))) // General case
	numNodes := 1 << uint(power)
	if power < 0 || numNodes <= 0 {
		panic("bad power")
	}
	// Allocate all nodes.
	graph := make([]*node2, numNodes)
	for i := range graph {
		graph[i] = new(node2)
	}
	// Create edges.
	for i, node := range graph {
		// Interpret i as a number in base "degree"; drop the
		// leading digit, shift left a digit, and for each
		// possible value of the new right-most digit, link to
		// that node.
		next := i * degree % numNodes
		for digit := 0; digit < degree; digit++ {
			node.ptrs[digit] = graph[next+digit]
		}
	}

	// Hold on to just one node. By construction, all nodes are
	// reachable from any node.
	return graph[0]
}
