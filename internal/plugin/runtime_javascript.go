package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	shared "res-downloader/internal/model"
	"time"

	"github.com/dop251/goja"
)

const (
	maxPluginScriptSize = 1024 * 1024
	pluginExecutionTime = 5 * time.Second
)

type javaScriptPlugin struct {
	manifest shared.PluginManifest
	program  *goja.Program
	filename string
	services pluginRuntimeServices
}

func newJavaScriptPlugin(directory string, manifest shared.PluginManifest, configured ...pluginRuntimeServices) (shared.RuntimePlugin, error) {
	services := pluginRuntimeServices{correlations: newPluginCorrelationStore()}
	if len(configured) > 0 {
		services = configured[0]
		if services.correlations == nil {
			services.correlations = newPluginCorrelationStore()
		}
	}
	entryAbs, err := securePluginFilePath(directory, manifest.Entry)
	if err != nil {
		return nil, fmt.Errorf("resolve JavaScript entry: %w", err)
	}
	info, err := os.Stat(entryAbs)
	if err != nil {
		return nil, fmt.Errorf("read JavaScript entry: %w", err)
	}
	if info.Size() > maxPluginScriptSize {
		return nil, fmt.Errorf("JavaScript entry exceeds %d bytes", maxPluginScriptSize)
	}
	source, err := os.ReadFile(entryAbs)
	if err != nil {
		return nil, err
	}
	program, err := goja.Compile(entryAbs, string(source), true)
	if err != nil {
		return nil, fmt.Errorf("compile JavaScript: %w", err)
	}
	return &javaScriptPlugin{manifest: manifest, program: program, filename: entryAbs, services: services}, nil
}

func (p *javaScriptPlugin) Manifest() shared.PluginManifest { return p.manifest }

func (p *javaScriptPlugin) Handle(ctx context.Context, obs shared.Observation) (shared.PluginResult, error) {
	result := shared.PluginResult{}
	emitted := make([]shared.ResourceCandidate, 0)
	value, called, err := p.call(ctx, "onObservation", obs, p.apiFactory(&emitted, obs.Settings))
	if err != nil {
		return result, err
	}
	if called && len(value) > 0 {
		if err := json.Unmarshal(value, &result); err != nil {
			return result, fmt.Errorf("export onObservation result: %w", err)
		}
	}
	result.Resources = append(result.Resources, emitted...)
	return result, nil
}

func (p *javaScriptPlugin) HandlePageMessage(ctx context.Context, message interface{}, pageContext shared.PageMessageContext) (shared.PageMessageResult, bool, error) {
	result := shared.PageMessageResult{}
	emitted := make([]shared.ResourceCandidate, 0)
	value, called, err := p.callArguments(ctx, "onPageMessage", []interface{}{message, pageContext}, p.apiFactory(&emitted, pageContext.Settings))
	if err != nil || !called || len(value) == 0 {
		return result, called, err
	}
	if err := json.Unmarshal(value, &result); err != nil {
		return result, true, fmt.Errorf("export onPageMessage result: %w", err)
	}
	result.Resources = append(result.Resources, emitted...)
	return result, true, nil
}

func (p *javaScriptPlugin) baseAPI(settings map[string]interface{}) func(*goja.Runtime) *goja.Object {
	enableLog, _ := settings["enableLog"].(bool)
	return func(vm *goja.Runtime) *goja.Object {
		api := vm.NewObject()
		_ = api.Set("log", func(call goja.FunctionCall) goja.Value {
			if enableLog && p.services.logger != nil {
				p.services.logger.Info().Msgf("plugin %s: %s", p.manifest.ID, call.Argument(0).String())
			}
			return goja.Undefined()
		})
		_ = api.Set("pluginVersion", p.manifest.Version)
		return api
	}
}

func (p *javaScriptPlugin) apiFactory(emitted *[]shared.ResourceCandidate, settings map[string]interface{}) func(*goja.Runtime) *goja.Object {
	base := p.baseAPI(settings)
	return func(vm *goja.Runtime) *goja.Object {
		api := base(vm)
		emit := func(call goja.FunctionCall) goja.Value {
			var candidate shared.ResourceCandidate
			if exportJSON(call.Argument(0), &candidate) == nil {
				*emitted = append(*emitted, candidate)
			}
			return goja.Undefined()
		}
		_ = api.Set("emit", emit)
		_ = api.Set("upsert", emit)
		correlate := vm.NewObject()
		_ = correlate.Set("register", func(call goja.FunctionCall) goja.Value {
			var registration pluginCorrelationRegistration
			if exportJSON(call.Argument(0), &registration) == nil {
				p.services.correlations.register(p.manifest.ID, registration)
			}
			return goja.Undefined()
		})
		_ = correlate.Set("find", func(call goja.FunctionCall) goja.Value {
			return vm.ToValue(jsonValue(p.services.correlations.find(p.manifest.ID, call.Argument(0).String())))
		})
		_ = api.Set("correlate", correlate)
		if p.manifest.Permissions.Has("page-bridge") {
			page := vm.NewObject()
			_ = page.Set("send", func(call goja.FunctionCall) goja.Value {
				if p.services.pages == nil {
					return vm.ToValue(false)
				}
				var message interface{}
				if exportJSON(call.Argument(1), &message) != nil {
					return vm.ToValue(false)
				}
				return vm.ToValue(p.services.pages.send(p.manifest.ID, call.Argument(0).String(), message) == nil)
			})
			_ = page.Set("broadcast", func(call goja.FunctionCall) goja.Value {
				if p.services.pages == nil {
					return vm.ToValue(0)
				}
				filter := map[string]interface{}{}
				var message interface{}
				if exportJSON(call.Argument(0), &filter) != nil || exportJSON(call.Argument(1), &message) != nil {
					return vm.ToValue(0)
				}
				return vm.ToValue(p.services.pages.broadcast(p.manifest.ID, filter, message))
			})
			_ = page.Set("sessions", func(call goja.FunctionCall) goja.Value {
				if p.services.pages == nil {
					return vm.ToValue([]interface{}{})
				}
				filter := map[string]interface{}{}
				if !goja.IsUndefined(call.Argument(0)) && !goja.IsNull(call.Argument(0)) {
					if exportJSON(call.Argument(0), &filter) != nil {
						return vm.ToValue([]interface{}{})
					}
				}
				return vm.ToValue(p.services.pages.list(p.manifest.ID, filter))
			})
			_ = api.Set("page", page)
		}
		return api
	}
}

func (p *javaScriptPlugin) Resolve(ctx context.Context, resource shared.ResourceCandidate, options shared.DownloadOptions) (shared.DownloadPlan, bool, error) {
	if resource.Source.PluginID != p.manifest.ID {
		return shared.DownloadPlan{}, false, nil
	}
	argument := map[string]interface{}{"resource": resource, "options": options}
	value, called, err := p.call(ctx, "createDownloadPlan", argument, p.baseAPI(options.Settings))
	if err != nil || !called || len(value) == 0 {
		return shared.DownloadPlan{}, false, err
	}
	var plan shared.DownloadPlan
	if err := json.Unmarshal(value, &plan); err != nil {
		return plan, false, fmt.Errorf("export createDownloadPlan result: %w", err)
	}
	return plan, true, nil
}

func (p *javaScriptPlugin) RefreshResource(ctx context.Context, resource shared.ResourceCandidate, options shared.DownloadOptions) (shared.ResourceRefreshResult, bool, error) {
	if resource.Source.PluginID != p.manifest.ID {
		return shared.ResourceRefreshResult{}, false, nil
	}
	argument := map[string]interface{}{"resource": resource, "options": options}
	value, called, err := p.call(ctx, "refreshResource", argument, p.baseAPI(options.Settings))
	if err != nil || !called || len(value) == 0 {
		return shared.ResourceRefreshResult{}, called, err
	}
	var result shared.ResourceRefreshResult
	if err := json.Unmarshal(value, &result); err != nil {
		return result, true, fmt.Errorf("export refreshResource result: %w", err)
	}
	return result, true, nil
}

func (p *javaScriptPlugin) call(
	ctx context.Context,
	name string,
	argument interface{},
	apiFactory func(*goja.Runtime) *goja.Object,
) (json.RawMessage, bool, error) {
	return p.callArguments(ctx, name, []interface{}{argument}, apiFactory)
}

func (p *javaScriptPlugin) callArguments(
	ctx context.Context,
	name string,
	input []interface{},
	apiFactory func(*goja.Runtime) *goja.Object,
) (raw json.RawMessage, called bool, err error) {
	vm := goja.New()
	callCtx, cancel := context.WithTimeout(ctx, pluginExecutionTime)
	defer cancel()
	interruptDone := make(chan struct{})
	stopInterrupt := context.AfterFunc(callCtx, func() {
		defer close(interruptDone)
		vm.Interrupt(pluginCallContextError(callCtx))
	})
	defer func() {
		if !stopInterrupt() {
			<-interruptDone
		}
	}()
	// Export can execute getters outside a normal JS function call. Goja's
	// uncatchable interrupts must also become errors at this boundary.
	defer func() {
		if recovered := recover(); recovered != nil {
			switch failure := recovered.(type) {
			case *goja.InterruptedError:
				raw, err = nil, fmt.Errorf("call %s: %w", name, failure)
			default:
				panic(recovered)
			}
		}
	}()
	if callCtx.Err() != nil {
		return nil, false, pluginCallContextError(callCtx)
	}
	if exception := vm.Try(func() {
		if _, err = vm.RunProgram(p.program); err != nil {
			err = fmt.Errorf("initialise %s: %w", p.filename, err)
			return
		}
		function, ok := goja.AssertFunction(vm.Get(name))
		if !ok {
			return
		}
		arguments := make([]goja.Value, 0, len(input)+1)
		for _, argument := range input {
			arguments = append(arguments, vm.ToValue(jsonValue(argument)))
		}
		if apiFactory != nil {
			arguments = append(arguments, apiFactory(vm))
		}
		called = true
		var value goja.Value
		value, err = function(goja.Undefined(), arguments...)
		if err != nil {
			err = fmt.Errorf("call %s: %w", name, err)
			return
		}
		if value != nil && !goja.IsUndefined(value) && !goja.IsNull(value) {
			// Keep the VM interrupt active through getters and JSON encoding.
			// No live VM value escapes to the caller after this scope closes.
			raw, err = json.Marshal(value.Export())
			if err != nil {
				err = fmt.Errorf("export %s result: %w", name, err)
			}
		}
	}); exception != nil {
		return nil, called, fmt.Errorf("call %s: %w", name, exception)
	}
	if callCtx.Err() != nil {
		return nil, called, pluginCallContextError(callCtx)
	}
	return raw, called, err
}

func jsonValue(value interface{}) interface{} {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var out interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	return out
}

func exportJSON(value goja.Value, target interface{}) error {
	raw, err := json.Marshal(value.Export())
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, target)
}
