package packager

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/internal/utils"
	"github.com/defenseunicorns/zarf/src/types"
)


func loopScriptUntilSuccess(script string, scripts types.ZarfComponentScripts) {
	spinner := message.NewProgressSpinner("Waiting for command \"%s\"", script)
	defer spinner.Stop()

	// Try to patch the zarf binary path in case the name isn't exactly "./zarf"
	binaryPath, err := os.Executable()
	if err != nil {
		spinner.Errorf(err, "Unable to determine the current zarf binary path")
	} else {
		script = strings.ReplaceAll(script, "./zarf ", binaryPath+" ")
	}

	var ctx context.Context
	var cancel context.CancelFunc

	// Default timeout is 5 minutes
	if scripts.TimeoutSeconds < 1 {
		scripts.TimeoutSeconds = 300
	}

	duration := time.Duration(scripts.TimeoutSeconds) * time.Second
	timeout := time.After(duration)

	spinner.Updatef("Waiting for command \"%s\" (timeout: %d seconds)", script, scripts.TimeoutSeconds)

	for {
		select {
		// On timeout abort
		case <-timeout:
			cancel()
			spinner.Fatalf(nil, "Script \"%s\" timed out", script)
		// Oherwise try running the script
		default:
			ctx, cancel = context.WithTimeout(context.Background(), duration)
			output, errOut, err := utils.ExecCommandWithContext(ctx, scripts.ShowOutput, "sh", "-c", script)
			defer cancel()

			if err != nil {
				message.Debug(err, output, errOut)
				// If retry, let the script run again
				if scripts.Retry {
					continue
				}
				// Otherwise fatal
				spinner.Fatalf(err, "Script \"%s\" did complete sucessfully: %s", script, err.Error())
			}

			// Dump the script output in debug if output not already streamed
			if !scripts.ShowOutput {
				message.Debug(output, errOut)
			}

			// Close the function now that we are done
			spinner.Success()
			return
		}
	}
}