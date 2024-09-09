// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// forked from https://www.socketloop.com/tutorials/golang-byte-format-example

// Package utils provides generic utility functions.
package utils

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/zarf-dev/zarf/src/pkg/message"
)

type Unit struct {
	name string
	size float64
}

var (
	gigabyte = Unit{
		name: "GB",
		size: 1000000000,
	}
	megabyte = Unit{
		name: "MB",
		size: 1000000,
	}
	kilobyte = Unit{
		name: "KB",
		size: 1000,
	}
	unitByte = Unit{
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

// ByteFormat formats a number of bytes into a human readable string.
func ByteFormat(in float64, precision int) string {
	if precision <= 0 {
		precision = 1
	}

	var unit string
	var val float64

	// https://www.techtarget.com/searchstorage/definition/mebibyte-MiB
	switch {
	case gigabyte.size <= in:
		val = RoundUp(in/gigabyte.size, precision)
		unit = gigabyte.name
		break
	case 1000000 <= in:
		val = RoundUp(in/1000000, precision)
		unit = megabyte.name
		break
	case 1000 <= in:
		val = RoundUp(in/1000, precision)
		unit = kilobyte.name
		break
	default:
		val = in
		unit = unitByte.name
		break
	}

	// NOTE(mkcp): Negative bytes are nonsense, but it's more robust for inputs without erroring.
	if val < -1 || 1 < val {
		unit += "s"
	}

	vFmt := strconv.FormatFloat(val, 'f', precision, 64)
	return vFmt + " " + unit
}

// RenderProgressBarForLocalDirWrite creates a progress bar that continuously tracks the progress of writing files to a local directory and all of its subdirectories.
// NOTE: This function runs infinitely until either completeChan or errChan is triggered, this function should be run in a goroutine while a different thread/process is writing to the directory.
func RenderProgressBarForLocalDirWrite(filepath string, expectedTotal int64, completeChan chan error, updateText string, successText string) {
	// Create a progress bar
	title := fmt.Sprintf("%s (%s of %s)", updateText, ByteFormat(float64(0), 2), ByteFormat(float64(expectedTotal), 2))
	progressBar := message.NewProgressBar(expectedTotal, title)

	for {
		select {
		case err := <-completeChan:
			if err == nil {
				// Send success message
				progressBar.Successf("%s (%s)", successText, ByteFormat(float64(expectedTotal), 2))
				completeChan <- nil
				return
			} else {
				if err := progressBar.Close(); err != nil {
					message.Debugf("unable to close progress bar: %s", err.Error())
				}
				completeChan <- nil
				return
			}
		default:
			// Read the directory size
			currentBytes, dirErr := helpers.GetDirSize(filepath)
			if dirErr != nil {
				message.Debugf("unable to get updated progress: %s", dirErr.Error())
				time.Sleep(200 * time.Millisecond)
				continue
			}

			// Update the progress bar with the current size
			title := fmt.Sprintf("%s (%s of %s)", updateText, ByteFormat(float64(currentBytes), 2), ByteFormat(float64(expectedTotal), 2))
			progressBar.Update(currentBytes, title)
			time.Sleep(200 * time.Millisecond)
		}
	}
}
