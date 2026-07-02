package parser

import (
	"encoding/binary"
	"fmt"
	"io"
)

type Operator byte

const (
	Equal Operator = iota
	NotEqual
	GreaterThan
	LessThan
	GreaterThanOrEqual
	LessThanOrEqual
)

type tester func(io.ReaderAt, string) (bool, string, error)
type offsetReader func(io.ReaderAt) (int64, error)

type MagicTest struct {
	Test     tester
	Message  string
	Children []MagicTest
}

func StringTest(offsetFunc offsetReader, compare string) tester {
	return func(r io.ReaderAt, pattern string) (bool, string, error) {
		b := make([]byte, len(compare))
		offset, err := offsetFunc(r)
		if err != nil {
			return false, "", err
		}
		n, err := r.ReadAt(b, offset)
		if err != nil {
			return false, "", err
		}
		if n != len(compare) {
			return false, "", nil
		}
		isMatch := string(b) == compare
		if !isMatch {
			return false, "", nil
		}
		return isMatch, messageParser(r, offset, pattern), nil
	}
}

func ShortTestLittleEndian(offsetFunc offsetReader, compare uint16, comparator Operator) tester {
	return shortTest(offsetFunc, compare, comparator, binary.LittleEndian)
}

func ShortTestBigEndian(offsetFunc offsetReader, compare uint16, comparator Operator) tester {
	return shortTest(offsetFunc, compare, comparator, binary.BigEndian)
}

func shortTest(offsetFunc offsetReader, compare uint16, comparator Operator, endian binary.ByteOrder) tester {
	return func(r io.ReaderAt, pattern string) (bool, string, error) {
		b := make([]byte, 2)
		offset, err := offsetFunc(r)
		if err != nil {
			return false, "", err
		}
		n, err := r.ReadAt(b, offset)
		if err != nil {
			return false, "", err
		}
		if n != len(b) {
			return false, "", nil
		}
		actual := endian.Uint16(b)
		var isMatch bool
		switch comparator {
		case Equal:
			isMatch = actual == compare
		case NotEqual:
			isMatch = actual != compare
		case GreaterThan:
			isMatch = actual > compare
		case LessThan:
			isMatch = actual < compare
		case GreaterThanOrEqual:
			isMatch = actual >= compare
		case LessThanOrEqual:
			isMatch = actual <= compare
		default:
			return false, "", fmt.Errorf("unknown comparator %d", comparator)
		}
		if !isMatch {
			return false, "", nil
		}
		return isMatch, messageParser(r, offset, pattern), nil
	}
}

func LongTestLittleEndian(offsetFunc offsetReader, compare uint64, comparator Operator) tester {
	return longTest(offsetFunc, compare, comparator, binary.LittleEndian)
}

func LongTestBigEndian(offsetFunc offsetReader, compare uint64, comparator Operator) tester {
	return longTest(offsetFunc, compare, comparator, binary.BigEndian)
}

func longTest(offsetFunc offsetReader, compare uint64, comparator Operator, endian binary.ByteOrder) tester {
	return func(r io.ReaderAt, pattern string) (bool, string, error) {
		b := make([]byte, 8)
		offset, err := offsetFunc(r)
		if err != nil {
			return false, "", err
		}
		n, err := r.ReadAt(b, offset)
		if err != nil {
			return false, "", err
		}
		if n != len(b) {
			return false, "", nil
		}
		actual := endian.Uint64(b)
		var isMatch bool
		switch comparator {
		case Equal:
			isMatch = actual == compare
		case NotEqual:
			isMatch = actual != compare
		case GreaterThan:
			isMatch = actual > compare
		case LessThan:
			isMatch = actual < compare
		case GreaterThanOrEqual:
			isMatch = actual >= compare
		case LessThanOrEqual:
			isMatch = actual <= compare
		default:
			return false, "", fmt.Errorf("unknown comparator %d", comparator)
		}
		if !isMatch {
			return false, "", nil
		}
		return isMatch, messageParser(r, offset, pattern), nil
	}
}

func ByteTest(offsetFunc offsetReader, compare byte, comparator Operator) tester {
	return func(r io.ReaderAt, pattern string) (bool, string, error) {
		b := make([]byte, 1)
		offset, err := offsetFunc(r)
		if err != nil {
			return false, "", err
		}
		n, err := r.ReadAt(b, offset)
		if err != nil {
			return false, "", err
		}
		if n != len(b) {
			return false, "", nil
		}
		var isMatch bool
		actual := b[0]
		switch comparator {
		case Equal:
			isMatch = actual == compare
		case NotEqual:
			isMatch = actual != compare
		case GreaterThan:
			isMatch = actual > compare
		case LessThan:
			isMatch = actual < compare
		case GreaterThanOrEqual:
			isMatch = actual >= compare
		case LessThanOrEqual:
			isMatch = actual <= compare
		default:
			return false, "", fmt.Errorf("unknown comparator %d", comparator)
		}
		if !isMatch {
			return false, "", nil
		}
		return isMatch, messageParser(r, offset, pattern), nil
	}
}
