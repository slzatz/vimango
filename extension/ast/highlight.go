// Package ast defines AST nodes that represents extension's elements
package ast

import (
	gast "github.com/yuin/goldmark/ast"
)

// A Highlight struct represents an html mark.
type Highlight struct {
	gast.BaseInline
}

// Dump implements Node.Dump.
func (n *Highlight) Dump(source []byte, level int) {
	gast.DumpHelper(n, source, level, nil, nil)
}

// KindHighlight is a NodeKind of the Highlight node.
var KindHighlight = gast.NewNodeKind("Highlight")

// Kind implements Node.Kind.
func (n *Highlight) Kind() gast.NodeKind {
	return KindHighlight
}

// NewHighlight returns a new Highlight node.
func NewHighlight() *Highlight {
	return &Highlight{}
}
