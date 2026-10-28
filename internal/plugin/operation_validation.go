package plugin

import (
	"encoding/json"
	"fmt"
	m "res-downloader/internal/model"
	"res-downloader/internal/operation"
)

func validateOperations(manifest m.PluginManifest) error {
	if len(manifest.Operations) > 32 {
		return fmt.Errorf("more than 32 operations")
	}
	for id, op := range manifest.Operations {
		if !validIdentifier(id) || len(op.Name) == 0 || len(op.Name) > 128 || len(op.Description) > 2048 {
			return fmt.Errorf("invalid operation identity")
		}
		if manifest.Runtime != "javascript" || !manifest.Permissions.Has("page-bridge") {
			return fmt.Errorf("operations require javascript and page-bridge")
		}
		found := false
		for _, script := range manifest.PageScripts {
			if script.ID == op.PageScript && script.Bridge {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("operation %s references an unavailable page script", id)
		}
		if op.TimeoutSeconds < 0 || op.TimeoutSeconds > int(operation.MaxTimeout.Seconds()) || op.ResultTTLSeconds < 0 || op.ResultTTLSeconds > int(operation.ResultTTL.Seconds()) {
			return fmt.Errorf("operation %s exceeds timeout or retention bounds", id)
		}
		switch op.Category {
		case "search", "detail", "current-content", "list", "resolve", "text", "custom":
		default:
			return fmt.Errorf("invalid operation category")
		}
		if len(op.Effects) == 0 || len(op.Effects) > 4 {
			return fmt.Errorf("operation effects are required")
		}
		seen := map[string]bool{}
		for _, effect := range op.Effects {
			if seen[effect] {
				return fmt.Errorf("duplicate operation effect")
			}
			seen[effect] = true
			switch effect {
			case "read", "page":
			case "publish":
				if !manifest.Permissions.Has("emit-resource") {
					return fmt.Errorf("publish requires emit-resource")
				}
			case "download":
				if !manifest.Permissions.Has("emit-resource") || !manifest.Permissions.Has("enqueue-download") {
					return fmt.Errorf("download requires emit-resource and enqueue-download")
				}
			default:
				return fmt.Errorf("invalid operation effect")
			}
		}
		if op.SafeRetry && (seen["page"] || seen["download"] || seen["publish"]) {
			return fmt.Errorf("safeRetry requires read-only effects")
		}
		if op.AllowReload && !seen["page"] {
			return fmt.Errorf("allowReload requires page effects")
		}
		for _, match := range op.PageMatch {
			if match.Host == "" || !pageScriptDomainAllowed(manifest.Permissions.Domains, match.Host) {
				return fmt.Errorf("operation pageMatch is outside permissions")
			}
		}
		for _, schema := range []map[string]interface{}{op.InputSchema, op.OutputSchema} {
			raw, err := json.Marshal(schema)
			if err != nil || len(raw) > 16*1024 {
				return fmt.Errorf("operation schema too large")
			}
			if err := operation.ValidateSchema(schema); err != nil {
				return fmt.Errorf("operation %s: %w", id, err)
			}
		}
		if op.InputSchema["type"] != "object" {
			return fmt.Errorf("operation input must be an object")
		}
		if len(op.Examples) > 4 {
			return fmt.Errorf("too many operation examples")
		}
		for _, example := range op.Examples {
			if err := operation.ValidateValue(op.InputSchema, example); err != nil {
				return err
			}
		}
	}
	return nil
}
