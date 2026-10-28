package plugin

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"res-downloader/internal/capture"
	shared "res-downloader/internal/model"
	"strconv"
	"strings"
	"testing"
)

func captureRuntime(t *testing.T, id, script string, store PageCaptureStore, permitted bool) shared.RuntimePlugin {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.js"), []byte(script), 0600); err != nil {
		t.Fatal(err)
	}
	manifest := shared.PluginManifest{ID: id, Name: "Capture fixture", Version: "1", APIVersion: 1, Runtime: "javascript", Entry: "main.js"}
	if permitted {
		manifest.Permissions.Capabilities = []string{"capture-response-body"}
	}
	p, err := newJavaScriptPlugin(dir, manifest, pluginRuntimeServices{captureStore: func() PageCaptureStore { return store }})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPluginCaptureSaveBytes(t *testing.T) {
	store, err := capture.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, tc := range []struct {
		input string
		want  []byte
	}{
		{`"标题\n😀\u0000正文"`, []byte("标题\n😀\x00正文")},
		{`new Uint8Array([9,0,255,65,8]).subarray(1,4)`, []byte{0, 255, 65}},
		{`new Uint8Array([0,1,255]).buffer`, []byte{0, 1, 255}},
		{`new Uint8ClampedArray([0,255])`, []byte{0, 255}},
	} {
		t.Run(tc.input, func(t *testing.T) {
			script := `function onObservation(o,api) {var f=api.capture.save(` + tc.input + `);return {diagnostics:[f.captureKey,String(f.size)]};}`
			p := captureRuntime(t, "test.capture", script, store, true)
			previous := ""
			for range 2 {
				r, err := p.Handle(context.Background(), shared.Observation{})
				if err != nil || len(r.Diagnostics) != 2 {
					t.Fatalf("save: %#v %v", r, err)
				}
				key := r.Diagnostics[0]
				if key == previous || !strings.HasPrefix(key, "saved:") || r.Diagnostics[1] != strconv.Itoa(len(tc.want)) {
					t.Fatalf("invalid save result: %#v", r.Diagnostics)
				}
				previous = key
				reader, _, err := store.OpenComplete(scopedCaptureKey("test.capture", key))
				if err != nil {
					t.Fatal(err)
				}
				got, err := io.ReadAll(reader)
				reader.Close()
				if err != nil || !bytes.Equal(got, tc.want) {
					t.Fatalf("bytes = %v, error = %v", got, err)
				}
				if _, _, err := store.OpenComplete(scopedCaptureKey("test.other", key)); err == nil {
					t.Fatal("capture escaped plugin namespace")
				}
			}
		})
	}
}

type captureFailureStore struct {
	fail                        string
	started, aborted, completed int
	size                        int64
}

func (s *captureFailureStore) StartStream(string) error {
	s.started++
	if s.fail == "start" {
		return errors.New("private filesystem path")
	}
	return nil
}
func (s *captureFailureStore) AppendStream(_ string, b []byte) (int64, error) {
	if s.fail == "write" {
		return 0, errors.New("private filesystem path")
	}
	if s.fail == "short" {
		return int64(len(b) - 1), nil
	}
	s.size = int64(len(b))
	return s.size, nil
}
func (s *captureFailureStore) CompleteStream(string) error {
	if s.fail == "complete" {
		return errors.New("private filesystem path")
	}
	s.completed++
	return nil
}
func (s *captureFailureStore) AbortStream(string) error { s.aborted++; return nil }

func TestPluginCaptureSaveValidation(t *testing.T) {
	for _, input := range []string{`undefined`, `null`, `123`, `[]`, `[1,2]`, `{get bytes(){throw "getter"}}`, `new Uint16Array([1])`, `""`, `new ArrayBuffer(0)`, `new Uint8Array(33554433)`, `"中".repeat(12000000)`} {
		t.Run(input, func(t *testing.T) {
			store := &captureFailureStore{}
			p := captureRuntime(t, "test.capture", `function onObservation(o,api){api.capture.save(`+input+`);}`, store, true)
			if _, err := p.Handle(context.Background(), shared.Observation{}); err == nil || store.started != 0 {
				t.Fatalf("invalid data reached store: %v", err)
			}
		})
	}
	for _, tc := range []struct {
		name, script string
		count        int
	}{
		{"count", `for(var i=0;i<9;i++)api.capture.save("a");`, 8},
		{"bytes", `api.capture.save(new Uint8Array(33554432));api.capture.save("a");`, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &captureFailureStore{}
			p := captureRuntime(t, "test.capture", `function onObservation(o,api){`+tc.script+`}`, store, true)
			for attempt := 1; attempt <= 2; attempt++ {
				if _, err := p.Handle(context.Background(), shared.Observation{}); err == nil || store.started != tc.count*attempt {
					t.Fatalf("per-hook limit failed: starts=%d err=%v", store.started, err)
				}
			}
		})
	}
}

func TestPluginCaptureSaveFailuresAndPermission(t *testing.T) {
	for _, stage := range []string{"start", "write", "short", "complete"} {
		store := &captureFailureStore{fail: stage}
		p := captureRuntime(t, "test.capture", `function onObservation(o,api){api.capture.save("a");}`, store, true)
		_, err := p.Handle(context.Background(), shared.Observation{})
		if err == nil || strings.Contains(err.Error(), "private filesystem path") || store.aborted != 1 || store.completed != 0 {
			t.Fatalf("%s: err=%v store=%#v", stage, err, store)
		}
	}
	p := captureRuntime(t, "test.capture", `function onObservation(o,api){if(api.capture!==undefined)throw "permission leak";}`, nil, false)
	if _, err := p.Handle(context.Background(), shared.Observation{}); err != nil {
		t.Fatal(err)
	}
	p = captureRuntime(t, "test.capture", `function onObservation(o,api){api.capture.save("a");}`, nil, true)
	if _, err := p.Handle(context.Background(), shared.Observation{}); err == nil || !strings.Contains(err.Error(), "capture store is unavailable") {
		t.Fatalf("unavailable store: %v", err)
	}
}

func TestPluginCaptureSavePageMessage(t *testing.T) {
	store := &captureFailureStore{}
	p := captureRuntime(t, "test.capture", `function onPageMessage(m,c,api){return {ok:true,data:api.capture.save("ok")};}`, store, true)
	r, called, err := p.(shared.PageMessageHandler).HandlePageMessage(context.Background(), nil, shared.PageMessageContext{})
	if err != nil || !called || !r.OK || store.completed != 1 {
		t.Fatalf("page hook: %#v %v", r, err)
	}
}
