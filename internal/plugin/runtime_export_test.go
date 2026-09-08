package plugin

import (
	"context"
	"errors"
	"fmt"
	shared "res-downloader/internal/model"
	"testing"
	"time"

	"github.com/dop251/goja"
)

func TestJavaScriptResultGettersRespectDeadlineAndCancellation(t *testing.T) {
	for _, hook := range []string{"onObservation", "onPageMessage", "createDownloadPlan", "refreshResource"} {
		for _, cancelOnly := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/cancel=%v", hook, cancelOnly), func(t *testing.T) {
				program, err := goja.Compile("getter.js", `function `+hook+`() {
  return {get diagnostics() { while (true) {} }};
}`, true)
				if err != nil {
					t.Fatal(err)
				}
				plugin := &javaScriptPlugin{program: program, filename: "getter.js", manifest: shared.PluginManifest{ID: "test.getter"}}
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
				want := context.DeadlineExceeded
				if cancelOnly {
					cancel()
					ctx, cancel = context.WithCancel(context.Background())
					timer := time.AfterFunc(30*time.Millisecond, cancel)
					defer timer.Stop()
					want = context.Canceled
				}
				defer cancel()
				finished := make(chan error, 1)
				go func() {
					resource := shared.ResourceCandidate{Source: shared.ResourceSource{PluginID: plugin.manifest.ID}}
					var callErr error
					switch hook {
					case "onObservation":
						_, callErr = plugin.Handle(ctx, shared.Observation{})
					case "onPageMessage":
						_, _, callErr = plugin.HandlePageMessage(ctx, nil, shared.PageMessageContext{})
					case "createDownloadPlan":
						_, _, callErr = plugin.Resolve(ctx, resource, shared.DownloadOptions{})
					case "refreshResource":
						_, _, callErr = plugin.RefreshResource(ctx, resource, shared.DownloadOptions{})
					}
					finished <- callErr
				}()
				select {
				case err := <-finished:
					if !errors.Is(err, want) {
						t.Fatalf("getter error = %v, want %v", err, want)
					}
				case <-time.After(2 * time.Second):
					t.Fatal("getter continued after cancellation/deadline")
				}
			})
		}
	}
}

func TestJavaScriptAllowsHookLongerThanPreviousLimit(t *testing.T) {
	program, err := goja.Compile("slow.js", `function onObservation() {
  var started = Date.now();
  while (Date.now() - started < 650) {}
  return {decision: "continue"};
}`, true)
	if err != nil {
		t.Fatal(err)
	}
	plugin := &javaScriptPlugin{program: program, filename: "slow.js"}
	result, err := plugin.Handle(context.Background(), shared.Observation{})
	if err != nil || result.Decision != shared.DecisionContinue {
		t.Fatalf("previous 500ms limit still affects hook: result=%#v err=%v", result, err)
	}
}
