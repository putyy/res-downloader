package plugin

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	shared "res-downloader/internal/model"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type PluginFixture struct {
	Observation  shared.Observation   `json:"observation" yaml:"observation"`
	Observations []shared.Observation `json:"observations,omitempty" yaml:"observations,omitempty"`
	Expected     FixtureExpected      `json:"expected" yaml:"expected"`
}

type FixtureExpected struct {
	ResourceCount     *int     `json:"resourceCount,omitempty" yaml:"resourceCount,omitempty"`
	ResourceURLs      []string `json:"resourceUrls,omitempty" yaml:"resourceUrls,omitempty"`
	ProcessorTypes    []string `json:"processorTypes,omitempty" yaml:"processorTypes,omitempty"`
	Decision          string   `json:"decision,omitempty" yaml:"decision,omitempty"`
	PatchBodyContains string   `json:"patchBodyContains,omitempty" yaml:"patchBodyContains,omitempty"`
}

func ValidatePluginDirectory(directory string) (shared.PluginManifest, error) {
	// Authoring tools may validate official.* plugins. Trust is assigned later by
	// the store index or bundled-plugin loader, never by linting or packing.
	runtime, _, err := LoadOfficialPlugin(directory)
	if err != nil {
		return shared.PluginManifest{}, err
	}
	return runtime.Manifest(), nil
}

func ValidateBundledPluginDirectory(directory string) (shared.PluginManifest, error) {
	runtime, _, err := LoadBundledPlugin(directory)
	if err != nil {
		return shared.PluginManifest{}, err
	}
	return runtime.Manifest(), nil
}

func ReplayPluginFixture(directory, fixturePath string) error {
	runtime, _, err := LoadOfficialPlugin(directory)
	if err != nil {
		return err
	}
	return replayPluginFixture(runtime, fixturePath)
}

func replayPluginFixture(runtime shared.RuntimePlugin, fixturePath string) error {
	raw, err := os.ReadFile(fixturePath)
	if err != nil {
		return err
	}
	var fixture PluginFixture
	if strings.HasSuffix(fixturePath, ".json") {
		err = json.Unmarshal(raw, &fixture)
	} else {
		err = yaml.Unmarshal(raw, &fixture)
	}
	if err != nil {
		return fmt.Errorf("parse fixture: %w", err)
	}
	observations := fixture.Observations
	if len(observations) == 0 {
		observations = []shared.Observation{fixture.Observation}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result := shared.PluginResult{Decision: shared.DecisionContinue}
	resourceIndexes := make(map[string]int)
	for observationIndex, rawObservation := range observations {
		if !pluginMatches(runtime.Manifest(), rawObservation) {
			return fmt.Errorf("fixture observation %d does not match plugin manifest", observationIndex)
		}
		rawObservation.Settings = effectivePluginSettings(runtime.Manifest().SettingsSchema, rawObservation.Settings)
		observation := sanitizeObservation(rawObservation, runtime.Manifest().Permissions)
		if !pluginWantsBody(runtime, runtime.Manifest(), rawObservation) {
			observation.Request.Body = ""
			if observation.Response != nil {
				observation.Response.Body = ""
			}
		}
		current, callErr := runtime.Handle(ctx, observation)
		if callErr != nil {
			return callErr
		}
		result.Decision = current.Decision
		result.Patch = current.Patch
		result.Captures = append(result.Captures, current.Captures...)
		for _, resource := range current.Resources {
			key := resource.GroupKey
			if key != "" {
				if index, exists := resourceIndexes[key]; exists {
					result.Resources[index] = mergeResourceCandidate(result.Resources[index], resource)
					continue
				}
				resourceIndexes[key] = len(result.Resources)
			}
			result.Resources = append(result.Resources, resource)
		}
	}
	for index := range result.Resources {
		result.Resources[index].Source.PluginID = runtime.Manifest().ID
		if err := validateResourceActions(runtime.Manifest(), result.Resources[index].Actions); err != nil {
			return fmt.Errorf("validate fixture resource %d actions: %w", index, err)
		}
		if err := validateCandidate(&result.Resources[index]); err != nil {
			return fmt.Errorf("validate fixture resource %d: %w", index, err)
		}
	}
	if expected := fixture.Expected.ResourceCount; expected != nil && len(result.Resources) != *expected {
		return fmt.Errorf("resource count is %d, expected %d", len(result.Resources), *expected)
	}
	for _, expectedURL := range fixture.Expected.ResourceURLs {
		found := false
		for _, resource := range result.Resources {
			for _, track := range resource.Tracks {
				if track.URL == expectedURL {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			return fmt.Errorf("expected resource URL %q was not emitted", expectedURL)
		}
	}
	if len(fixture.Expected.ProcessorTypes) > 0 {
		if len(result.Resources) == 0 {
			return errors.New("expected download processors, but the plugin emitted no resources")
		}
		resource := result.Resources[0]
		plan, handled, err := runtime.Resolve(ctx, resource, shared.DownloadOptions{})
		if err != nil {
			return fmt.Errorf("resolve fixture resource: %w", err)
		}
		if !handled {
			plan = fallbackDownloadPlan(resource, shared.DownloadOptions{})
		}
		plan = normalizeDownloadPlan(plan)
		if err := validateDownloadPlan(plan); err != nil {
			return fmt.Errorf("validate fixture download plan: %w", err)
		}
		processors := append([]shared.DownloadStep(nil), plan.Output.Processors...)
		for _, input := range plan.Inputs {
			processors = append(processors, input.Processors...)
		}
		if len(processors) != len(fixture.Expected.ProcessorTypes) {
			return fmt.Errorf("processor count is %d, expected %d", len(processors), len(fixture.Expected.ProcessorTypes))
		}
		for index, expectedType := range fixture.Expected.ProcessorTypes {
			if processors[index].Type != expectedType {
				return fmt.Errorf("processor %d is %q, expected %q", index, processors[index].Type, expectedType)
			}
		}
	}
	if fixture.Expected.Decision != "" && result.Decision != fixture.Expected.Decision {
		return fmt.Errorf("decision is %q, expected %q", result.Decision, fixture.Expected.Decision)
	}
	if expected := fixture.Expected.PatchBodyContains; expected != "" {
		if result.Patch == nil || result.Patch.Body == nil || !strings.Contains(*result.Patch.Body, expected) {
			return fmt.Errorf("response patch does not contain %q", expected)
		}
	}
	return nil
}

func RunPluginCLI(args []string, output io.Writer) error {
	if len(args) < 2 {
		return errors.New("usage: res-downloader plugin <create|lint|lint-bundled|replay|pack|sync-bundled> ...")
	}
	switch args[0] {
	case "create":
		id := filepath.Base(args[1])
		if len(args) > 2 {
			id = args[2]
		}
		if err := createPluginScaffold(args[1], id); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(output, "created plugin %s in %s\n", id, args[1])
		return nil
	case "lint":
		manifest, err := ValidatePluginDirectory(args[1])
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(output, "valid plugin %s v%s (API %d, %s)\n", manifest.ID, manifest.Version, manifest.APIVersion, manifest.Runtime)
		return nil
	case "lint-bundled":
		manifest, err := ValidateBundledPluginDirectory(args[1])
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(output, "valid bundled plugin %s v%s (API %d, %s)\n", manifest.ID, manifest.Version, manifest.APIVersion, manifest.Runtime)
		return nil
	case "replay":
		if len(args) < 3 {
			return errors.New("usage: res-downloader plugin replay <plugin-directory> <fixture.json|yaml>")
		}
		if err := ReplayPluginFixture(args[1], args[2]); err != nil {
			return err
		}
		_, _ = fmt.Fprintln(output, "fixture passed")
		return nil
	case "pack":
		outputPath := filepath.Join(args[1], "dist", "plugin.zip")
		if len(args) > 2 {
			outputPath = args[2]
		}
		if err := packPluginDirectory(args[1], outputPath); err != nil {
			return err
		}
		_, _ = fmt.Fprintln(output, outputPath)
		return nil
	case "sync-bundled":
		manifest, target, err := syncBundledPlugin(args[1])
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(output, "synced %s v%s to %s\n", manifest.ID, manifest.Version, target)
		return nil
	default:
		return fmt.Errorf("unknown plugin command %q", args[0])
	}
}

func createPluginScaffold(directory, id string) error {
	if !validIdentifier(id) || reservedPluginIDPrefix(id) != "" {
		return errors.New("invalid plugin id")
	}
	if entries, err := os.ReadDir(directory); err == nil && len(entries) > 0 {
		return errors.New("plugin directory is not empty")
	}
	if err := os.MkdirAll(filepath.Join(directory, "fixtures"), 0750); err != nil {
		return err
	}
	manifest := fmt.Sprintf(`{
  "id": %q,
  "name": %q,
  "author": {"name": "Your Name", "url": "https://github.com/your-name"},
  "version": "1.0.0",
  "apiVersion": 1,
  "runtime": "javascript",
  "entry": "main.js",
  "permissions": {"domains": ["example.com"], "capabilities": ["observe-response", "emit-resource"]},
  "match": [{"stage": "response", "host": "example.com"}]
}
`, id, id)
	script := `function onObservation(observation, api) {
  return {decision: "continue", resources: []};
}

function createDownloadPlan(input) {
  return null;
}
`
	if err := os.WriteFile(filepath.Join(directory, "plugin.json"), []byte(manifest), 0644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(directory, "main.js"), []byte(script), 0644)
}

func packPluginDirectory(directory, outputPath string) error {
	if _, err := ValidatePluginDirectory(directory); err != nil {
		return err
	}
	directoryAbsolute, err := filepath.Abs(directory)
	if err != nil {
		return err
	}
	outputAbsolute, err := filepath.Abs(outputPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(outputAbsolute), 0750); err != nil {
		return err
	}
	output, err := os.Create(outputAbsolute)
	if err != nil {
		return err
	}
	archive := zip.NewWriter(output)
	walkErr := filepath.WalkDir(directoryAbsolute, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != directoryAbsolute {
				relative, relativeErr := filepath.Rel(directoryAbsolute, path)
				if relativeErr != nil {
					return relativeErr
				}
				first := strings.SplitN(filepath.ToSlash(relative), "/", 2)[0]
				if excludedPluginDevelopmentDirectory(first) {
					return filepath.SkipDir
				}
			}
			return nil
		}
		pathAbsolute, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		if pathAbsolute == outputAbsolute {
			return nil
		}
		relative, err := filepath.Rel(directoryAbsolute, path)
		if err != nil {
			return err
		}
		writer, err := archive.Create(filepath.ToSlash(relative))
		if err != nil {
			return err
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(writer, file)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
	archiveErr := archive.Close()
	closeErr := output.Close()
	if walkErr != nil {
		return walkErr
	}
	if archiveErr != nil {
		return archiveErr
	}
	return closeErr
}

func excludedPluginDevelopmentDirectory(name string) bool {
	return name == ".git" || name == "dist" || name == "tests"
}
