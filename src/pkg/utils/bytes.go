// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
// forked from https://www.socketloop.com/tutorials/golang-byte-format-example

// Package utils provides generic helper functions.
package utils

import (
	"fmt"
	"math"
	"strconv"
	"sync"
	"time"

	"github.com/defenseunicorns/zarf/src/pkg/message"
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

// CheckLocalProgress continuously checks the local directory size and updates the progress bar.
// NOTE: This function runs infinitely until the completeChan is triggered.
func CheckLocalProgress(progressBar *message.ProgressBar, expectedTotal int64, filepath string, wg *sync.WaitGroup, completeChan chan int, updateText string) {
	for {
		select {
		case <-completeChan:
			// Fill out the progress bar so that it's a little less choppy
			title := fmt.Sprintf("%s (%s of %s)", updateText, ByteFormat(float64(expectedTotal), 2), ByteFormat(float64(expectedTotal), 2))
			progressBar.Update(int64(expectedTotal), title)

			// Send success message
			progressBar.Successf("%s (%s)", updateText, ByteFormat(float64(expectedTotal), 2))
			wg.Done()
			return

		default:
			// Read the directory size
			currentBytes, dirErr := GetDirSize(filepath)
			if dirErr != nil {
				message.Warnf("unable to get the updated progress of the image pull: %s", dirErr.Error())
				time.Sleep(200 * time.Millisecond)
				continue
			}

			// Update the progress bar with the current size
			title := fmt.Sprintf("%s (%s of %s)", updateText, ByteFormat(float64(expectedTotal), 2), ByteFormat(float64(currentBytes), 2))
			progressBar.Update(currentBytes, title)
			time.Sleep(200 * time.Millisecond)
		}
	}
}
