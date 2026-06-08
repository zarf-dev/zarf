// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package template provides functions for applying go-templates within Zarf.
package template

import (
	ttmpl "text/template"
	"text/template/parse"
)

// Refs are the system-object references discovered in a template string. Each Values entry is
// the slice of path segments following ".Values" (e.g. ".Values.db.host" -> {"db", "host"}).
// Variables and Constants hold the names following ".Variables" and ".Constants".
type Refs struct {
	Values    [][]string
	Variables []string
	Constants []string
}

// ReferencedKeys parses s as a go-template and returns the .Values, .Variables, and .Constants
// references it makes. It does not execute the template. References whose root dot has been
// rebound (inside range/with) are not resolved back to the system objects and are ignored, so
// the result may undercount but never overcounts. Unparseable templates return an error.
func ReferencedKeys(s string) (Refs, error) {
	tmpl, err := ttmpl.New("refs").Funcs(funcMap()).Parse(s)
	if err != nil {
		return Refs{}, err
	}
	refs := Refs{}
	for _, t := range tmpl.Templates() {
		if t.Tree == nil || t.Tree.Root == nil {
			continue
		}
		walkNode(t.Tree.Root, &refs)
	}
	return refs, nil
}

func walkNode(n parse.Node, refs *Refs) {
	switch node := n.(type) {
	case *parse.ListNode:
		if node == nil {
			return
		}
		for _, child := range node.Nodes {
			walkNode(child, refs)
		}
	case *parse.ActionNode:
		walkNode(node.Pipe, refs)
	case *parse.PipeNode:
		if node == nil {
			return
		}
		for _, cmd := range node.Cmds {
			walkNode(cmd, refs)
		}
	case *parse.CommandNode:
		for _, arg := range node.Args {
			walkNode(arg, refs)
		}
	case *parse.FieldNode:
		recordField(node.Ident, refs)
	case *parse.ChainNode:
		walkNode(node.Node, refs)
	case *parse.IfNode:
		walkBranch(node.Pipe, node.List, node.ElseList, refs)
	case *parse.RangeNode:
		walkBranch(node.Pipe, node.List, node.ElseList, refs)
	case *parse.WithNode:
		walkBranch(node.Pipe, node.List, node.ElseList, refs)
	case *parse.TemplateNode:
		walkNode(node.Pipe, refs)
	}
}

func walkBranch(pipe *parse.PipeNode, list, elseList *parse.ListNode, refs *Refs) {
	walkNode(pipe, refs)
	walkNode(list, refs)
	walkNode(elseList, refs)
}

func recordField(ident []string, refs *Refs) {
	if len(ident) == 0 {
		return
	}
	switch ident[0] {
	case objectKeyValues:
		if len(ident) > 1 {
			refs.Values = append(refs.Values, append([]string(nil), ident[1:]...))
		}
	case objectKeyVariables:
		if len(ident) > 1 {
			refs.Variables = append(refs.Variables, ident[1])
		}
	case objectKeyConstants:
		if len(ident) > 1 {
			refs.Constants = append(refs.Constants, ident[1])
		}
	}
}
