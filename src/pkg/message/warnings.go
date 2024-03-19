// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package message provides a rich set of functions for displaying messages to the user.
package message

type Warnings struct {
	messages []string
}

func NewWarnings() *Warnings {
	return &Warnings{}
}

func (w *Warnings) Add(messages ...string) {
	w.messages = append(w.messages, messages...)
}

func (w *Warnings) HasWarnings() bool {
	return len(w.messages) > 0
}

func (w *Warnings) GetMessages() []string {
	return w.messages
}
