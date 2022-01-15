package utils

import (
	"math"
	"strconv"
)

// forked from https://www.socketloop.com/tutorials/golang-byte-format-example

func RoundUp(input float64, places int) (newVal float64) {
	var round float64
	pow := math.Pow(10, float64(places))
	digit := pow * input
	round = math.Ceil(digit)
	newVal = round / pow
	return
}

func ByteFormat(inputNum float64, precision int) string {
	if precision <= 0 {
		precision = 1
	}

	var unit string
	var returnVal float64

	if inputNum >= 1000000000 {
		returnVal = RoundUp(inputNum/1073741824, precision)
		unit = " GB" // gigabyte
	} else if inputNum >= 1000000 {
		returnVal = RoundUp(inputNum/1048576, precision)
		unit = " MB" // megabyte
	} else if inputNum >= 1000 {
		returnVal = RoundUp(inputNum/1024, precision)
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
