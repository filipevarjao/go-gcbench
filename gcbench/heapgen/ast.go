// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package heapgen

import (
	"go/parser"
	"go/token"
	"log"
)

// MakeAST generates garbage by parsing the net/http source code. Each
// AST is ~1.8MB of heap.
func MakeAST() interface{} {
	ast, err := parser.ParseFile(token.NewFileSet(), "net/http", src, parser.ParseComments)
	if err != nil {
		log.Fatal(err)
	}
	return ast
}
