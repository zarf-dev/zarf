package magic

import (
	"io"

	"github.com/deitch/magic/pkg/magic/internal"
	parser "github.com/deitch/magic/pkg/magic/parser"
)

// TODO: clone the sources from https://github.com/file/file/blob/master/magic/Magdir
// or even the compiled versions from https://pkgs.alpinelinux.org/package/edge/main/x86_64/libmagic

func GetType(r io.ReaderAt) ([]string, error) {
	for _, m := range internal.AllTests {
		results, err := getType(r, m)
		if err != nil {
			return nil, err
		} else if len(results) != 0 {
			return results, nil
		}
	}
	return nil, nil
}

func getType(r io.ReaderAt, test parser.MagicTest) ([]string, error) {
	var results []string
	if ok, message, err := test.Test(r, test.Message); err != nil {
		return nil, err
	} else if !ok {
		return nil, nil
	} else if message != "" {
		results = append(results, message)
	}
	if len(test.Children) > 0 {
		for _, child := range test.Children {
			subResults, err := getType(r, child)
			if err != nil {
				return nil, err
			}
			results = append(results, subResults...)
		}
	}
	return results, nil
}
