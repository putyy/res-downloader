package updates

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseRequiresSuccessfulCleanup(t *testing.T) {
	for _, failed := range []bool{false, true} {
		name := "success"
		var shutdownErr error
		if failed {
			name = "failure"
			shutdownErr = errors.New("close HTTP gateway: deadline exceeded")
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			gate := filepath.Join(root, "continue")
			m := &Manager{root: root, job: &Job{Gate: gate}}
			if err := m.Release(shutdownErr); err != nil {
				t.Fatal(err)
			}
			_, gateErr := os.Stat(gate)
			_, cancelErr := os.Stat(m.job.cancellationFile())
			failure, readErr := os.ReadFile(filepath.Join(root, "last-error.txt"))
			if failed {
				if cancelErr != nil {
					t.Fatalf("failed cleanup did not stop its helper: %v", cancelErr)
				}
				if !errors.Is(gateErr, os.ErrNotExist) {
					t.Fatalf("failed cleanup allowed installation: %v", gateErr)
				}
				if readErr != nil || !strings.Contains(string(failure), shutdownErr.Error()) {
					t.Fatalf("cleanup failure not retained: %q, %v", failure, readErr)
				}
			} else {
				if !errors.Is(cancelErr, os.ErrNotExist) {
					t.Fatalf("successful cleanup cancelled the helper: %v", cancelErr)
				}
				if gateErr != nil {
					t.Fatalf("successful cleanup did not release helper: %v", gateErr)
				}
				if !errors.Is(readErr, os.ErrNotExist) {
					t.Fatalf("unexpected cleanup failure: %q, %v", failure, readErr)
				}
			}
		})
	}
}

func TestCancelledHelperDoesNotRepeatConsumedFailure(t *testing.T) {
	root := t.TempDir()
	job := &Job{Gate: filepath.Join(root, "continue")}
	m := &Manager{root: root, job: job}
	if err := m.Release(errors.New("cleanup failed")); err != nil {
		t.Fatal(err)
	}
	// A new launch consumes last-error.txt; the old job must still stop,
	// whether or not the previous process has finished exiting yet.
	report := filepath.Join(root, "last-error.txt")
	if err := os.Remove(report); err != nil {
		t.Fatal(err)
	}
	for _, running := range []bool{true, false} {
		ready, err := shutdownReady(*job, running)
		if ready || !errors.Is(err, errShutdownCancelled) {
			t.Fatalf("cancelled helper kept waiting or installed: ready=%t, err=%v", ready, err)
		}
	}
	if _, err := os.Stat(report); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("consumed failure was recreated: %v", err)
	}
}
