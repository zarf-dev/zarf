// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package transform

// Log is a function that logs a message.
// TODO(mkcp): Remove Log and port over to logger once we remove message.
type Log func(string, ...any)
