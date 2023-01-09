// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package utils provides generic helper functions
package utils

import (
	"time"
)

// Unique returns a new slice with only unique elements
func Unique[T comparable](s []T) []T {
	exists := make(map[T]bool)
	var result []T
	for _, str := range s {
		if _, ok := exists[str]; !ok {
			exists[str] = true
			result = append(result, str)
		}
	}
	return result
}

// Retry will retry a function until it succeeds or the timeout is reached, timeout == retries * delay
func Retry(fn func() error, retries int, delay time.Duration) (err error) {
	for r := 0; r < retries; r++ {
		err = fn()
		if err == nil {
			break
		}

		time.Sleep(delay)
	}

	return err
}

// Insert returns a new slice with the element inserted at the given index
func Insert[T any](slice []T, index int, element T) []T {
	if len(slice) == index {
			return append(slice, element)
	}
	slice = append(slice[:index+1], slice[index:]...)
	slice[index] = element
	return slice
}