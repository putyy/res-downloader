package proxy

import (
	"os/exec"
	"testing"
)

// This runs only when a maintainer explicitly executes the Go tests. The normal
// static validation workflow compiles this package without executing the harness.
func TestOperationSDKAsyncLifecycle(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js is required for the asynchronous page SDK regression tests")
	}
	command := exec.Command(node, "--test", "testdata/operation_sdk.test.cjs")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("page SDK asynchronous regression tests: %v\n%s", err, output)
	}
}
