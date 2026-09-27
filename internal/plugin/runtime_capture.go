package plugin

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"reflect"
	shared "res-downloader/internal/model"

	"github.com/dop251/goja"
)

const (
	maxPluginCaptureFiles = 8
	maxPluginCaptureBytes = 32 * 1024 * 1024
)

// Each API instance belongs to one hook invocation, not to a persistent VM.
// save deliberately has no caller-supplied key/path: it cannot overwrite a
// response capture, a page upload, or a file currently used by another hook.
func (p *javaScriptPlugin) captureAPI(vm *goja.Runtime) *goja.Object {
	api := vm.NewObject()
	count, total := 0, 0
	_ = api.Set("save", func(call goja.FunctionCall) goja.Value {
		if count >= maxPluginCaptureFiles {
			panic(vm.NewTypeError("capture file count exceeds the per-hook limit"))
		}
		data, err := pluginCaptureBytes(call.Argument(0), maxPluginCaptureBytes-total)
		if err != nil {
			panic(vm.NewTypeError("%s", err))
		}
		var store PageCaptureStore
		if p.services.captureStore != nil {
			store = p.services.captureStore()
		}
		if store == nil {
			panic(vm.NewGoError(errors.New("capture store is unavailable")))
		}
		// Count attempted writes as well, so caught I/O failures cannot bypass limits.
		count++
		total += len(data)
		var nonce [16]byte
		if _, err := rand.Read(nonce[:]); err != nil {
			panic(vm.NewGoError(errors.New("capture key could not be created")))
		}
		key := "saved:" + hex.EncodeToString(nonce[:])
		scoped := scopedCaptureKey(p.manifest.ID, key)
		// Roll back an incomplete write on any failure, without exposing paths or
		// store implementation errors to the plugin.
		complete := false
		defer func() {
			if !complete {
				_ = store.AbortStream(scoped)
			}
		}()
		if err := store.StartStream(scoped); err != nil {
			panic(vm.NewGoError(errors.New("capture could not be started")))
		}
		written, err := store.AppendStream(scoped, data)
		if err != nil || written != int64(len(data)) {
			panic(vm.NewGoError(errors.New("capture could not be written")))
		}
		if err := store.CompleteStream(scoped); err != nil {
			panic(vm.NewGoError(errors.New("capture could not be completed")))
		}
		complete = true
		return vm.ToValue(jsonValue(shared.PluginCaptureFile{CaptureKey: key, Size: written}))
	})
	return api
}

func pluginCaptureBytes(value goja.Value, limit int) ([]byte, error) {
	var data []byte
	switch value.ExportType() {
	case reflect.TypeOf(""):
		// Bound UTF-16 length before allocating UTF-8; then check actual bytes.
		if str, ok := value.(goja.String); !ok || str.Length() > limit {
			return nil, errors.New("capture bytes exceed the per-hook limit")
		}
		data = []byte(value.String())
	case reflect.TypeOf([]byte(nil)):
		data = value.Export().([]byte) // Uint8Array/Uint8ClampedArray, respecting view bounds.
	case reflect.TypeOf(goja.ArrayBuffer{}):
		data = value.Export().(goja.ArrayBuffer).Bytes()
	default:
		return nil, errors.New("capture data must be a string, ArrayBuffer or byte typed array")
	}
	if len(data) == 0 || len(data) > limit {
		return nil, errors.New("capture data must be nonempty and within the per-hook byte limit")
	}
	return data, nil
}
