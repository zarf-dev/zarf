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
type Refs struct {
	Values [][]string
}

// ReferencedKeys parses s as a go-template and returns the .Values references it makes. It does not
// execute the template. Direct field access (.Values.db.host) and index access with string literals
// (index .Values "db" "host") are resolved. References that can only be resolved at execution time are
// not: a root dot rebound inside range/with, a dot stored in a variable ($v := .Values), or an index
// key that is not a string literal (index .Values $key). Those are silently ignored, so the result may
// undercount but never overcounts. Unparseable templates return an error.
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
		if path, ok := indexValuesPath(node); ok {
			refs.Values = append(refs.Values, path)
			return
		}
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

// indexValuesPath returns the .Values path addressed by an `index .Values ...` command when every
// index key is a string literal, e.g. `index .Values.db "host"` -> {"db", "host"}. It returns ok=false
// for any other command, or when an index key is not a string literal, since those resolve only at
// execution time.
func indexValuesPath(cmd *parse.CommandNode) ([]string, bool) {
	if len(cmd.Args) < 2 {
		return nil, false
	}
	ident, ok := cmd.Args[0].(*parse.IdentifierNode)
	if !ok || ident.Ident != "index" {
		return nil, false
	}
	field, ok := cmd.Args[1].(*parse.FieldNode)
	if !ok || len(field.Ident) == 0 || field.Ident[0] != objectKeyValues {
		return nil, false
	}
	path := append([]string(nil), field.Ident[1:]...)
	for _, arg := range cmd.Args[2:] {
		s, ok := arg.(*parse.StringNode)
		if !ok {
			return nil, false
		}
		path = append(path, s.Text)
	}
	if len(path) == 0 {
		return nil, false
	}
	return path, true
}

func recordField(ident []string, refs *Refs) {
	if len(ident) > 1 && ident[0] == objectKeyValues {
		refs.Values = append(refs.Values, append([]string(nil), ident[1:]...))
	}
}
