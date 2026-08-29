package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	shared "res-downloader/internal/model"
	"time"

	"github.com/dop251/goja"
)

const (
	maxPluginScriptSize = 1024 * 1024
	pluginExecutionTime = 500 * time.Millisecond
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
	value, called, err := p.call(ctx, "onObservation", obs, p.apiFactory(&emitted))
	if err != nil {
		return result, err
	}
	if called && value != nil && !goja.IsUndefined(value) && !goja.IsNull(value) {
		if err := exportJSON(value, &result); err != nil {
			return result, fmt.Errorf("export onObservation result: %w", err)
		}
	}
	result.Resources = append(result.Resources, emitted...)
	return result, nil
}

func (p *javaScriptPlugin) HandlePageMessage(ctx context.Context, message interface{}, pageContext shared.PageMessageContext) (shared.PageMessageResult, bool, error) {
	result := shared.PageMessageResult{}
	emitted := make([]shared.ResourceCandidate, 0)
	value, called, err := p.callArguments(ctx, "onPageMessage", []interface{}{message, pageContext}, p.apiFactory(&emitted))
	if err != nil || !called || value == nil || goja.IsUndefined(value) || goja.IsNull(value) {
		return result, called, err
	}
	if err := exportJSON(value, &result); err != nil {
		return result, true, fmt.Errorf("export onPageMessage result: %w", err)
	}
	result.Resources = append(result.Resources, emitted...)
	return result, true, nil
}

func (p *javaScriptPlugin) apiFactory(emitted *[]shared.ResourceCandidate) func(*goja.Runtime) *goja.Object {
	return func(vm *goja.Runtime) *goja.Object {
		api := vm.NewObject()
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
		_ = api.Set("log", func(call goja.FunctionCall) goja.Value {
			if p.services.logger != nil {
				p.services.logger.Info().Msgf("plugin %s: %s", p.manifest.ID, call.Argument(0).String())
			}
			return goja.Undefined()
		})
		_ = api.Set("pluginVersion", p.manifest.Version)
		return api
	}
}

func (p *javaScriptPlugin) Resolve(ctx context.Context, resource shared.ResourceCandidate, options shared.DownloadOptions) (shared.DownloadPlan, bool, error) {
	if resource.Source.PluginID != p.manifest.ID {
		return shared.DownloadPlan{}, false, nil
	}
	argument := map[string]interface{}{"resource": resource, "options": options}
	value, called, err := p.call(ctx, "createDownloadPlan", argument, nil)
	if err != nil || !called || value == nil || goja.IsUndefined(value) || goja.IsNull(value) {
		return shared.DownloadPlan{}, false, err
	}
	var plan shared.DownloadPlan
	if err := exportJSON(value, &plan); err != nil {
		return plan, false, fmt.Errorf("export createDownloadPlan result: %w", err)
	}
	return plan, true, nil
}

func (p *javaScriptPlugin) RefreshResource(ctx context.Context, resource shared.ResourceCandidate, options shared.DownloadOptions) (shared.ResourceRefreshResult, bool, error) {
	if resource.Source.PluginID != p.manifest.ID {
		return shared.ResourceRefreshResult{}, false, nil
	}
	argument := map[string]interface{}{"resource": resource, "options": options}
	value, called, err := p.call(ctx, "refreshResource", argument, nil)
	if err != nil || !called || value == nil || goja.IsUndefined(value) || goja.IsNull(value) {
		return shared.ResourceRefreshResult{}, called, err
	}
	var result shared.ResourceRefreshResult
	if err := exportJSON(value, &result); err != nil {
		return result, true, fmt.Errorf("export refreshResource result: %w", err)
	}
	return result, true, nil
}

func (p *javaScriptPlugin) call(
	ctx context.Context,
	name string,
	argument interface{},
	apiFactory func(*goja.Runtime) *goja.Object,
) (goja.Value, bool, error) {
	return p.callArguments(ctx, name, []interface{}{argument}, apiFactory)
}

func (p *javaScriptPlugin) callArguments(
	ctx context.Context,
	name string,
	input []interface{},
	apiFactory func(*goja.Runtime) *goja.Object,
) (goja.Value, bool, error) {
	vm := goja.New()
	deadline := pluginExecutionTime
	if contextDeadline, ok := ctx.Deadline(); ok {
		if remaining := time.Until(contextDeadline); remaining < deadline {
			deadline = remaining
		}
	}
	timer := time.AfterFunc(deadline, func() {
		vm.Interrupt(errors.New("plugin execution timed out"))
	})
	defer timer.Stop()
	if _, err := vm.RunProgram(p.program); err != nil {
		return nil, false, fmt.Errorf("initialise %s: %w", p.filename, err)
	}
	functionValue := vm.Get(name)
	function, ok := goja.AssertFunction(functionValue)
	if !ok {
		return nil, false, nil
	}
	arguments := make([]goja.Value, 0, len(input)+1)
	for _, argument := range input {
		arguments = append(arguments, vm.ToValue(jsonValue(argument)))
	}
	if apiFactory != nil {
		arguments = append(arguments, apiFactory(vm))
	}
	value, err := function(goja.Undefined(), arguments...)
	if err != nil {
		return nil, true, fmt.Errorf("call %s: %w", name, err)
	}
	return value, true, nil
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
