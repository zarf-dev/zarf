package test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDataInjection(t *testing.T) {

	// run `zarf init`
	err := e2e.execZarfCommand("init", "--confirm", "-l=trace")
	// output, err := exec.Command(e2e.zarfBinPath, "init", "--confirm", "-l=trace").CombinedOutput()
	assert.NoError(t, err, "unable to init")

	// Deploy the data injection example
	err = e2e.execZarfCommand("package", "deploy", "../../build/zarf-package-data-injection-demo.tar", "--confirm", "-l=trace")
	// output, err = exec.Command(e2e.zarfBinPath, "package", "deploy", "../../build/zarf-package-data-injection-demo.tar", "--confirm", "-l=trace").CombinedOutput()
	assert.NoError(t, err, "unable to deploy data-injection package")

	// time.Sleep(5 * time.Second)

	// Test to confirm the root file was placed
	var execStdOut string
	// var execStdErr string
	attempt := 0
	for attempt < 5 && execStdOut == "" {
		execStdOut, _, err = e2e.execCommandInPod("data-injection", "demo", []string{"ls", "/test"})
		attempt++
		// fmt.Printf("stdout after %v attempts:  %v\n", attempt, execStdOut)
		time.Sleep(2 * time.Second)
	}
	assert.NoError(t, err)
	assert.Contains(t, execStdOut, "subdirectory-test")

	attempt = 0
	execStdOut = ""

	// Test to confirm the subdirectory file was placed
	for attempt < 5 && execStdOut == "" {
		execStdOut, _, err = e2e.execCommandInPod("data-injection", "demo", []string{"ls", "/test/subdirectory-test"})
		attempt++
		// fmt.Printf("stdout after %v attempts:  %v\n", attempt, execStdOut)
		time.Sleep(2 * time.Second)
	}
	assert.NoError(t, err)
	assert.Contains(t, execStdOut, "this-is-an-example-file.txt")

	e2e.cleanupAfterTest(t)
}
