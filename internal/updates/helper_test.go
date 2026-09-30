package updates

import (
	"os"
	"path/filepath"
	"testing"
)

func TestShutdownRequiresGateAndExitedParent(t *testing.T) {
	for _, test := range []struct {
		name          string
		gate, running bool
		ready, failed bool
	}{
		{"waiting for cleanup", false, true, false, false},
		{"waiting for process", true, true, false, false},
		{"ready", true, false, true, false},
		{"exited without cleanup", false, false, false, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			job := Job{Gate: filepath.Join(t.TempDir(), "continue")}
			if test.gate {
				if err := os.WriteFile(job.Gate, []byte("ready"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			ready, err := shutdownReady(job, test.running)
			if ready != test.ready || (err != nil) != test.failed {
				t.Fatalf("ready=%t err=%v, want ready=%t failed=%t", ready, err, test.ready, test.failed)
			}
		})
	}
}
