/*
 * Copyright (c) SAS Institute, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package rpmutils

import (
	"regexp"
	"strings"
)

var (
	R_NONALNUMTILDE = regexp.MustCompile(`^([^a-zA-Z0-9~]*)(.*)$`)
	R_NUM           = regexp.MustCompile(`^([\d]+)(.*)$`)
	R_ALPHA         = regexp.MustCompile(`^([a-zA-Z]+)(.*)$`)
)

// VersionSlice provides the Sort interface for sorting version strings.
type VersionSlice []string

// Len is the number of elements in the collection.
func (vs VersionSlice) Len() int {
	return len(vs)
}

// Less reports wheather the element with index i should sort before the
// element with index j.
func (vs VersionSlice) Less(i, j int) bool {
	return Vercmp(vs[i], vs[j]) == -1
}

// Swap swaps the elements with indexes i and j.
func (vs VersionSlice) Swap(i, j int) {
	s1 := vs[i]
	vs[i] = vs[j]
	vs[j] = s1
}

// Vercmp compares two version strings using the same algorithm as rpm uses.
// Returns -1 if first < second, 1 if first > second, and 0 if first == second.
func Vercmp(first, second string) int {
	var m1Head, m2Head string
	for first != "" || second != "" {
		m1 := R_NONALNUMTILDE.FindStringSubmatch(first)
		m2 := R_NONALNUMTILDE.FindStringSubmatch(second)
		// This probably needs to return something different.
		if m1 == nil || m2 == nil {
			return 0
		}

		m1Head, first = m1[1], m1[2]
		m2Head, second = m2[1], m2[2]

		// Ignore junk at begining of version.
		if m1Head != "" || m2Head != "" {
			continue
		}

		// Hnalde the tolde seporator, it sorts before everything else.
		if strings.HasPrefix(first, "~") {
			if !strings.HasPrefix(second, "~") {
				return -1
			}
			first, second = first[1:], second[1:]
			continue
		}
		if strings.HasPrefix(second, "~") {
			return 1
		}

		// If we ran to the end of either, we are finished with the loop.
		if first == "" || second == "" {
			break
		}

		// Grab the first completely alpha or completely numeric segment.
		isnum := false
		m1 = R_NUM.FindStringSubmatch(first)
		m2 = R_NUM.FindStringSubmatch(second)
		if R_NUM.MatchString(first) {
			if !R_NUM.MatchString(second) {
				// numeric segments are always newer than alpha segments.
				return 1
			}
			isnum = true
		} else {
			m1 = R_ALPHA.FindStringSubmatch(first)
			m2 = R_ALPHA.FindStringSubmatch(second)
		}

		if len(m1) == 0 {
			// This condition should not be reached since we previously
			// tested to make sure that the first string has a non-nill
			// segment.
			return -1
		}
		if len(m2) == 0 {
			if isnum {
				return 1
			}
			return -1
		}

		m1Head, first = m1[1], m1[2]
		m2Head, second = m2[1], m2[2]

		if isnum {
			// Throw away any leading zeros.
			m1Head = strings.TrimLeft(m1Head, "0")
			m2Head = strings.TrimLeft(m2Head, "0")

			// Whichever number has more digits wins
			m1Hlen := len(m1Head)
			m2Hlen := len(m2Head)
			if m1Hlen < m2Hlen {
				return -1
			}
			if m1Hlen > m2Hlen {
				return 1
			}
		}

		// Same number of chars
		if m1Head < m2Head {
			return -1
		} else if m1Head > m2Head {
			return 1
		}

		// Both segments equal
		continue
	}

	m1Hlen := len(first)
	m2Hlen := len(second)
	if m1Hlen == 0 && m2Hlen == 0 {
		return 0
	}
	if m1Hlen != 0 {
		return 1
	}
	return -1
}
