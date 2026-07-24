// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"golang.org/x/telemetry"
	"golang.org/x/vuln/scan"
)

func main() {
	telemetry.Start(telemetry.Config{ReportCrashes: true})

	ctx := context.Background()

	cmd := scan.Command(ctx, os.Args[1:]...)
	err := cmd.Start()
	if err == nil {
		err = cmd.Wait()
	}
	if err != nil {
		var e interface{ ExitCode() int }
		if errors.As(err, &e) {
			printErrorToStderr := true
			if _, ok := err.(interface{ ExitCode() int }); ok {
				// Avoid printing the error to stderr if the exit code error wasn't
				// wrapped with another error providing context.
				printErrorToStderr = false
			}
			if printErrorToStderr {
				fmt.Fprintln(os.Stderr, err)
			}
			os.Exit(e.ExitCode())
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
