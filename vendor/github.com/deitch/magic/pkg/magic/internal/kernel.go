package internal

import (
	parser "github.com/deitch/magic/pkg/magic/parser"
)

func init() {
	AllTests = append(AllTests, kernelTests...)
}

var kernelTests = []parser.MagicTest{
	{Test: parser.StringTest(parser.WithOffset(514), "HdrS"), Message: "Linux kernel", Children: []parser.MagicTest{
		// the original for this is "leshort" - we are just treating this as a short, and would handle endianness when parsing
		// the magic file
		{Test: parser.ShortTestLittleEndian(parser.WithOffset(510), 0xAA55, parser.Equal), Message: "x86 boot executable", Children: []parser.MagicTest{
			{Test: parser.ShortTestLittleEndian(parser.WithOffset(518), 0x1ff, parser.GreaterThan), Children: []parser.MagicTest{
				{Test: parser.ByteTest(parser.WithOffset(529), 0, parser.Equal), Message: "zImage"},
				{Test: parser.ByteTest(parser.WithOffset(529), 1, parser.Equal), Message: "bzImage"},
				{Test: parser.LongTestLittleEndian(parser.WithOffset(526), 0, parser.GreaterThan), Children: []parser.MagicTest{
					{Test: parser.ByteTest(parser.WithChainedOffsetReaders(parser.WithIndirectOffsetShortLittleEndian(526), parser.WithOffset(0x200)), 0, parser.GreaterThan), Message: "version %s"},
				}},
			}},
			{Test: parser.ShortTestLittleEndian(parser.WithOffset(498), 1, parser.Equal), Message: "RO-rootFS"},
			{Test: parser.ShortTestLittleEndian(parser.WithOffset(498), 0, parser.Equal), Message: "RW-rootFS"},
			{Test: parser.ShortTestLittleEndian(parser.WithOffset(508), 0, parser.GreaterThan), Message: "root_dev %#X"},
			{Test: parser.ShortTestLittleEndian(parser.WithOffset(502), 0, parser.GreaterThan), Message: "swap_dev %#X"},
			{Test: parser.ShortTestLittleEndian(parser.WithOffset(504), 0, parser.GreaterThan), Message: "RAMdisksize %u KB"},
			{Test: parser.ShortTestLittleEndian(parser.WithOffset(506), 0xffff, parser.Equal), Message: "Normal VGA"},
			{Test: parser.ShortTestLittleEndian(parser.WithOffset(506), 0xfffe, parser.Equal), Message: "Extended VGA"},
			{Test: parser.ShortTestLittleEndian(parser.WithOffset(506), 0xfffd, parser.Equal), Message: "Prompt for Videomode"},
			{Test: parser.ShortTestLittleEndian(parser.WithOffset(506), 0x0, parser.GreaterThan), Message: "Video mode %d"},
		},
		},
	},
	}}
