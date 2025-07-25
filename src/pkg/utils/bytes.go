// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// forked from https://www.socketloop.com/tutorials/golang-byte-format-example

// Package utils provides generic utility functions.
package utils

import (
	"math"
	"strconv"
)

type unit struct {
	name string
	size float64
}

var (
	gigabyte = unit{
		name: "GB",
		size: 1000000000,
	}
	megabyte = unit{
		name: "MB",
		size: 1000000,
	}
	kilobyte = unit{
		name: "KB",
		size: 1000,
	}
	unitByte = unit{
		name: "Byte",
	}
)

// RoundUp rounds a float64 to the given number of decimal places.
func RoundUp(input float64, places int) float64 {
	pow := math.Pow(10, float64(places))
	digit := pow * input
	round := math.Ceil(digit)
	return round / pow
}

// ByteFormat formats a number of bytes into a human-readable string.
func ByteFormat(in float64, precision int) string {
	if precision <= 0 {
		precision = 1
	}

	var v float64
	var u string

	// https://www.techtarget.com/searchstorage/definition/mebibyte-MiB
	switch {
	case gigabyte.size <= in:
		v = RoundUp(in/gigabyte.size, precision)
		u = gigabyte.name
	case megabyte.size <= in:
		v = RoundUp(in/megabyte.size, precision)
		u = megabyte.name
	case kilobyte.size <= in:
		v = RoundUp(in/kilobyte.size, precision)
		u = kilobyte.name
	default:
		v = in
		u = unitByte.name
	}

	// NOTE(mkcp): Negative bytes are nonsense, but it's more robust for inputs without erroring.
	if v < -1 || 1 < v {
		u += "s"
	}

	vFmt := strconv.FormatFloat(v, 'f', precision, 64)
	return vFmt + " " + u
}
