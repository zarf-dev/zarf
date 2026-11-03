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
// execute the template. In Values, direct field access (.Values.db.host) and index access with string literals
// (index .Values "db" "host") are resolved. References that can only be resolved at execution time are
// not: a root dot rebound inside range/with, a dot stored in a variable ($v := .Values), or an index
// key that is not a string literal (index .Values $key). Those are silently ignored, so the result may
// undercount but never overcounts. It does not collect conservative value consumers.
// Unparseable templates return an error.
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
		walkBranch(&node.BranchNode, func(child parse.Node) { walkNode(child, refs) })
	case *parse.RangeNode:
		walkBranch(&node.BranchNode, func(child parse.Node) { walkNode(child, refs) })
	case *parse.WithNode:
		walkBranch(&node.BranchNode, func(child parse.Node) { walkNode(child, refs) })
	case *parse.TemplateNode:
		walkNode(node.Pipe, refs)
	}
}

// walkBranch shares the structural order without coupling each walk's policy.
func walkBranch(node *parse.BranchNode, visit func(parse.Node)) {
	visit(node.Pipe)
	visit(node.List)
	visit(node.ElseList)
}

// indexValuesPath returns the .Values path addressed by an `index .Values ...` command when every
// index key is a string literal, e.g. `index .Values.db "host"` -> {"db", "host"}.
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

// walkValueConsumers is opt-in so conservative consumption neither broadens nor adds work to
// the exact references used by deployment.
func walkValueConsumers(n parse.Node, consumers *[][]string) {
	if path, ok := consumerValuesPath(n); ok {
		*consumers = append(*consumers, path)
		return
	}
	switch node := n.(type) {
	case *parse.ListNode:
		if node != nil {
			for _, child := range node.Nodes {
				walkValueConsumers(child, consumers)
			}
		}
	case *parse.ActionNode:
		walkValueConsumers(node.Pipe, consumers)
	case *parse.PipeNode:
		if node != nil {
			for _, cmd := range node.Cmds {
				walkValueConsumers(cmd, consumers)
			}
		}
	case *parse.CommandNode:
		// Unresolved indexes still consume their source subtree and read every key expression.
		for _, arg := range node.Args {
			walkValueConsumers(arg, consumers)
		}
	case *parse.ChainNode:
		walkValueConsumers(node.Node, consumers)
	case *parse.IfNode:
		walkBranch(&node.BranchNode, func(child parse.Node) { walkValueConsumers(child, consumers) })
	case *parse.RangeNode:
		walkBranch(&node.BranchNode, func(child parse.Node) { walkValueConsumers(child, consumers) })
	case *parse.WithNode:
		walkBranch(&node.BranchNode, func(child parse.Node) { walkValueConsumers(child, consumers) })
	case *parse.TemplateNode:
		walkValueConsumers(node.Pipe, consumers)
	}
}

// consumerValuesPath resolves only expressions whose full path is statically known. Returning
// a precise index path prevents its source from also being counted as a broader consumer.
func consumerValuesPath(n parse.Node) ([]string, bool) {
	path, ok := consumerObjectPath(n)
	if !ok || len(path) == 0 || path[0] != objectKeyValues {
		return nil, false
	}
	return path[1:], true
}

// consumerObjectPath keeps the root object distinct from .Values so parenthesized root access
// such as ($).Values.db resolves without treating a bare dot as a values consumer.
func consumerObjectPath(n parse.Node) ([]string, bool) {
	switch node := n.(type) {
	case *parse.DotNode:
		return []string{}, true
	case *parse.FieldNode:
		return append([]string{}, node.Ident...), true
	case *parse.VariableNode:
		if len(node.Ident) > 0 && node.Ident[0] == "$" {
			return append([]string{}, node.Ident[1:]...), true
		}
	case *parse.ChainNode:
		if path, ok := consumerObjectPath(node.Node); ok {
			return append(path, node.Field...), true
		}
	case *parse.PipeNode:
		if node != nil && len(node.Cmds) == 1 {
			return consumerObjectPath(node.Cmds[0])
		}
	case *parse.CommandNode:
		if len(node.Args) == 1 {
			return consumerObjectPath(node.Args[0])
		}
		if len(node.Args) < 2 {
			return nil, false
		}
		ident, ok := node.Args[0].(*parse.IdentifierNode)
		if !ok || ident.Ident != "index" {
			return nil, false
		}
		path, ok := consumerObjectPath(node.Args[1])
		if !ok {
			return nil, false
		}
		for _, arg := range node.Args[2:] {
			key, ok := arg.(*parse.StringNode)
			if !ok {
				return nil, false
			}
			path = append(path, key.Text)
		}
		return path, true
	}
	return nil, false
}
