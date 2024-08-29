// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// forked from https://www.socketloop.com/tutorials/golang-byte-format-example

// Package utils provides generic utility functions.
package utils

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/zarf-dev/zarf/src/pkg/logging"
	"github.com/zarf-dev/zarf/src/pkg/message"
)

// RoundUp rounds a float64 to the given number of decimal places.
func RoundUp(input float64, places int) (newVal float64) {
	var round float64
	pow := math.Pow(10, float64(places))
	digit := pow * input
	round = math.Ceil(digit)
	newVal = round / pow
	return
}

// ByteFormat formats a number of bytes into a human readable string.
func ByteFormat(inputNum float64, precision int) string {
	if precision <= 0 {
		precision = 1
	}

	var unit string
	var returnVal float64

	// https://www.techtarget.com/searchstorage/definition/mebibyte-MiB
	if inputNum >= 1000000000 {
		returnVal = RoundUp(inputNum/1000000000, precision)
		unit = " GB" // gigabyte
	} else if inputNum >= 1000000 {
		returnVal = RoundUp(inputNum/1000000, precision)
		unit = " MB" // megabyte
	} else if inputNum >= 1000 {
		returnVal = RoundUp(inputNum/1000, precision)
		unit = " KB" // kilobyte
	} else {
		returnVal = inputNum
		unit = " Byte" // byte
	}

	if returnVal > 1 {
		unit += "s"
	}

	return strconv.FormatFloat(returnVal, 'f', precision, 64) + unit
}

// RenderProgressBarForLocalDirWrite creates a progress bar that continuously tracks the progress of writing files to a local directory and all of its subdirectories.
// NOTE: This function runs infinitely until either completeChan or errChan is triggered, this function should be run in a goroutine while a different thread/process is writing to the directory.
func RenderProgressBarForLocalDirWrite(ctx context.Context, filepath string, expectedTotal int64, completeChan chan error, updateText string, successText string) {
	log := logging.FromContextOrDiscard(ctx)

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
					log.Error("Unable to close progress bar", "error", err)
				}
				completeChan <- nil
				return
			}
		default:
			// Read the directory size
			currentBytes, dirErr := helpers.GetDirSize(filepath)
			if dirErr != nil {
				log.Error("Unable to get updated progress", "error", dirErr)
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
