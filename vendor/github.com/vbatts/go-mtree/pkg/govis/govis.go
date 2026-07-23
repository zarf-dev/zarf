// SPDX-License-Identifier: Apache-2.0
/*
 * govis: unicode aware vis(3) encoding implementation
 * Copyright (C) 2017-2025 SUSE LLC.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package govis

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// VisFlag manipulates how the characters are encoded/decoded
type VisFlag uint

// vis() has a variety of flags when deciding what encodings to use. While
// mtree only uses one set of flags, implementing them all is necessary in
// order to have compatibility with BSD's vis() and unvis() commands.
const (
	VisOctal       VisFlag = (1 << iota)     // VIS_OCTAL: Use octal \ddd format.
	VisCStyle                                // VIS_CSTYLE: Use \[nrft0..] where appropriate.
	VisSpace                                 // VIS_SP: Also encode space.
	VisTab                                   // VIS_TAB: Also encode tab.
	VisNewline                               // VIS_NL: Also encode newline.
	VisSafe                                  // VIS_SAFE: Encode unsafe characters.
	VisNoSlash                               // VIS_NOSLASH: Inhibit printing '\'.
	VisHTTPStyle                             // VIS_HTTPSTYLE: HTTP-style escape %xx.
	VisGlob                                  // VIS_GLOB: Encode glob(3) magics.
	VisDoubleQuote                           // VIS_DQ: Encode double-quotes (").
	visMask        VisFlag = (1 << iota) - 1 // Mask of all flags.

	VisWhite VisFlag = (VisSpace | VisTab | VisNewline)
)

// errUnknownVisFlagsError is a special value that lets you use [errors.Is]
// with [unknownVisFlagsError]. Don't actually return this value, use
// [unknownVisFlagsError] instead!
var errUnknownVisFlagsError = errors.New("unknown or unsupported vis flags")

// unknownVisFlagsError represents an error caused by unknown [VisFlag]s being
// passed to [Vis] or [Unvis].
type unknownVisFlagsError struct {
	flags VisFlag
}

func (err unknownVisFlagsError) Is(target error) bool {
	return target == errUnknownVisFlagsError
}

func (err unknownVisFlagsError) Error() string {
	return fmt.Sprintf("%s contains unknown or unsupported flags %s", err.flags, err.flags&^visMask)
}

// String pretty-prints VisFlag.
func (vflags VisFlag) String() string {
	flagNames := []struct {
		name string
		bits VisFlag
	}{
		{"VisOctal", VisOctal},
		{"VisCStyle", VisCStyle},
		{"VisSpace", VisSpace},
		{"VisTab", VisTab},
		{"VisNewline", VisNewline},
		{"VisSafe", VisSafe},
		{"VisNoSlash", VisNoSlash},
		{"VisHTTPStyle", VisHTTPStyle},
		{"VisGlob", VisGlob},
	}
	var (
		flagSet  = make([]string, 0, len(flagNames))
		seenBits VisFlag
	)
	for _, flag := range flagNames {
		if vflags&flag.bits == flag.bits {
			seenBits |= flag.bits
			flagSet = append(flagSet, flag.name)
		}
	}
	// If there were any remaining flags specified we don't know the name of,
	// just add them in an 0x... format.
	if remaining := vflags &^ seenBits; remaining != 0 {
		flagSet = append(flagSet, "0x"+strconv.FormatUint(uint64(remaining), 16))
	}
	if len(flagSet) == 0 {
		return "0"
	}
	return strings.Join(flagSet, "|")
}
