// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package template

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	ttmpl "text/template"
	"text/template/parse"
)

// ErrorKind identifies the phase in which a template failed.
type ErrorKind string

// Template error phases.
const (
	Syntax    ErrorKind = "syntax"
	Execution ErrorKind = "execution"
)

// Error describes a template failure without requiring callers to interpret Go's error text.
type Error struct {
	Kind ErrorKind
	// MissingInput is the executed field expression whose map lookup failed. Root "$" is
	// normalized to dot; aliases and rebound dots retain their authored spelling.
	// Supplied nils, type errors, function failures, and absent index keys do not set it.
	MissingInput string
	// Line and Column are one-based byte locations of the AST field node (which may
	// point within a chain), or zero when unavailable. Go's parser and generic
	// execution errors do not expose structured source locations.
	Line   int
	Column int
	Err    error
}

func (e *Error) Error() string { return e.Err.Error() }
func (e *Error) Unwrap() error { return e.Err }

// Template is a parsed template for opt-in source analysis and diagnostic execution.
// It retains native Go template control flow and missingkey=error behavior.
type Template struct {
	tmpl   *ttmpl.Template
	source string
}

// Parse parses s without executing it. Syntax failures are returned as *Error.
func Parse(s string) (*Template, error) {
	tmpl, err := ttmpl.New("str").Funcs(funcMap()).Option(missingKeyDefault).Parse(s)
	if err != nil {
		return nil, &Error{Kind: Syntax, Err: err}
	}
	return &Template{tmpl: tmpl, source: s}, nil
}

// ValueConsumers returns conservative .Values subtree consumers, including inactive
// branches. An empty path consumes all values. Aliases and rebound dots conservatively
// consume their source object. It does not change the exact references from ReferencedKeys.
func (t *Template) ValueConsumers() [][]string {
	var consumers [][]string
	for _, tmpl := range t.tmpl.Templates() {
		if tmpl.Tree != nil {
			walkValueConsumers(tmpl.Tree.Root, &consumers)
		}
	}
	return consumers
}

// Apply executes the template once, returning structured errors for missing map inputs.
// Unlike package-level Apply, it instruments field receivers for source diagnostics.
// Native index semantics are unchanged: an absent map index can yield a zero/no-value
// result, and is not by itself a missing-input error.
func (t *Template) Apply(_ context.Context, objs Objects) (string, error) {
	tmpl, err := t.tmpl.Clone()
	if err != nil {
		return "", &Error{Kind: Execution, Err: err}
	}
	// Register only after parsing, so authored templates cannot invoke the guard.
	tmpl.Funcs(ttmpl.FuncMap{lookupGuard: guardLookup})
	for _, associated := range tmpl.Templates() {
		if associated.Tree == nil {
			continue
		}
		// Clone shares parse trees. Copy before instrumentation so analysis and future
		// executions retain the original source, including concurrent executions.
		associated.Tree = associated.Tree.Copy()
		instrumentFields(associated.Tree.Root, t.source)
	}
	var b bytes.Buffer
	if err := tmpl.Execute(&b, objs); err != nil {
		diagnostic := &Error{Kind: Execution, Err: err}
		var missing *missingLookup
		if errors.As(err, &missing) {
			diagnostic.MissingInput = missing.input
			diagnostic.Line, diagnostic.Column = missing.line, missing.column
		}
		return "", diagnostic
	}
	return b.String(), nil
}

// lookupGuard names the private function injected into cloned analysis trees.
// It observes map receivers at execution time to attach typed missing-input
// metadata. Registering it after parsing keeps it out of authored templates.
const lookupGuard = "__zarf_template_lookup_guard"

type missingLookup struct {
	input        string
	key          string
	line, column int
}

func (e *missingLookup) Error() string { return fmt.Sprintf("map has no entry for key %q", e.key) }

// guardLookup observes only the lookup Go is about to perform, returning the original
// reflect.Value unchanged. Methods take precedence over map keys, just as in text/template.
// Nil, non-map, incompatible-key, and argument errors remain native execution errors.
func guardLookup(receiver reflect.Value, key, input string, line, column int) (reflect.Value, error) {
	v := receiver
	for v.IsValid() && (v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface) {
		if v.IsNil() {
			return receiver, nil
		}
		v = v.Elem()
	}
	if !v.IsValid() || v.Kind() != reflect.Map {
		return receiver, nil
	}
	ptr := v
	if ptr.CanAddr() {
		ptr = ptr.Addr()
	}
	if ptr.MethodByName(key).IsValid() {
		return receiver, nil
	}
	k := reflect.ValueOf(key)
	if k.Type().AssignableTo(v.Type().Key()) && !v.MapIndex(k).IsValid() {
		return receiver, &missingLookup{input: input, key: key, line: line, column: column}
	}
	return receiver, nil
}

func instrumentFields(n parse.Node, source string) {
	switch node := n.(type) {
	case *parse.ListNode:
		if node != nil {
			for _, child := range node.Nodes {
				instrumentFields(child, source)
			}
		}
	case *parse.ActionNode:
		instrumentFields(node.Pipe, source)
	case *parse.PipeNode:
		if node == nil {
			return
		}
		for i, cmd := range node.Cmds {
			for j, arg := range cmd.Args {
				// Go rejects map fields invoked with arguments before looking up their
				// key. Do not misclassify that failure as a missing input.
				hasArgs := j == 0 && (len(cmd.Args) > 1 || i > 0)
				cmd.Args[j] = guardFields(arg, source, hasArgs)
			}
		}
	case *parse.IfNode:
		walkBranch(&node.BranchNode, func(child parse.Node) { instrumentFields(child, source) })
	case *parse.WithNode:
		walkBranch(&node.BranchNode, func(child parse.Node) { instrumentFields(child, source) })
	case *parse.RangeNode:
		walkBranch(&node.BranchNode, func(child parse.Node) { instrumentFields(child, source) })
	case *parse.TemplateNode:
		instrumentFields(node.Pipe, source)
	}
}

func guardFields(n parse.Node, source string, hasArgs bool) parse.Node {
	var receiver parse.Node
	var fields []string
	input := n.String()
	if strings.HasPrefix(input, "$.") {
		input = input[1:]
	}
	switch node := n.(type) {
	case *parse.FieldNode:
		receiver = &parse.DotNode{NodeType: parse.NodeDot, Pos: node.Pos}
		fields = node.Ident
	case *parse.VariableNode:
		if len(node.Ident) < 2 {
			return n
		}
		root := *node
		root.Ident = node.Ident[:1]
		receiver, fields = &root, node.Ident[1:]
	case *parse.ChainNode:
		receiver, fields = guardFields(node.Node, source, false), node.Field
	default:
		instrumentFields(n, source)
		return n
	}
	pos := n.Position()
	prefix := source[:int(pos)]
	line := strings.Count(prefix, "\n") + 1
	column := len(prefix) - strings.LastIndex(prefix, "\n")
	for i, field := range fields {
		if !hasArgs || i < len(fields)-1 {
			receiver = &parse.PipeNode{NodeType: parse.NodePipe, Pos: pos, Cmds: []*parse.CommandNode{{
				NodeType: parse.NodeCommand, Pos: pos,
				Args: []parse.Node{
					parse.NewIdentifier(lookupGuard).SetPos(pos), receiver,
					&parse.StringNode{NodeType: parse.NodeString, Pos: pos, Text: field, Quoted: strconv.Quote(field)},
					&parse.StringNode{NodeType: parse.NodeString, Pos: pos, Text: input, Quoted: strconv.Quote(input)},
					&parse.NumberNode{NodeType: parse.NodeNumber, Pos: pos, IsInt: true, Int64: int64(line), Text: strconv.Itoa(line)},
					&parse.NumberNode{NodeType: parse.NodeNumber, Pos: pos, IsInt: true, Int64: int64(column), Text: strconv.Itoa(column)},
				},
			}}}
		}
		receiver = &parse.ChainNode{NodeType: parse.NodeChain, Pos: pos, Node: receiver, Field: []string{field}}
	}
	return receiver
}
