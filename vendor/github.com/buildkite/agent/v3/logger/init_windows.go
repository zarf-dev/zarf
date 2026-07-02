//go:build windows
// +build windows

package logger

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

// Windows 10 Build 16257 added support for ANSI color output if we enable them

func init() {
	var mode uint32
	stdout := windows.Handle(os.Stdout.Fd())

	if err := windows.GetConsoleMode(stdout, &mode); err != nil {
		return
	}

	// See https://docs.microsoft.com/en-us/windows/console/getconsolemode
	mode = mode | windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING

	if err := windows.SetConsoleMode(stdout, mode); err == nil {
		windowsColors = true
	} else {
		fmt.Printf("Error setting console mode: %v\n", err)
	}
}
