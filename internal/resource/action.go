package resource

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	shared "res-downloader/internal/model"
	"res-downloader/internal/naming"
	"strings"
)

func (r *Resource) processFileAction(
	candidate shared.ResourceCandidate,
	actionID string,
	definition shared.PluginActionDefinition,
	processor shared.DownloadStep,
	sourcePath string,
) {
	taskID := "action:" + candidate.ID + ":" + actionID
	if _, loaded := r.tasks.LoadOrStore(taskID, true); loaded {
		r.actionEventsEmit(candidate.ID, actionID, "error", "", "action is already running")
		return
	}
	defer r.tasks.Delete(taskID)

	outputPath, err := r.runFileAction(definition, processor, sourcePath)
	if err != nil {
		r.actionEventsEmit(candidate.ID, actionID, "error", "", err.Error())
		return
	}
	r.actionEventsEmit(candidate.ID, actionID, "done", outputPath, "complete")
}

func (r *Resource) ProcessFileAction(candidate shared.ResourceCandidate, actionID string, definition shared.PluginActionDefinition, processor shared.DownloadStep, sourcePath string) {
	r.processFileAction(candidate, actionID, definition, processor, sourcePath)
}

func (r *Resource) runFileAction(definition shared.PluginActionDefinition, processor shared.DownloadStep, sourcePath string) (string, error) {
	info, err := os.Stat(sourcePath)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("selected input is not a regular file")
	}
	inputExtension := strings.ToLower(filepath.Ext(sourcePath))
	if len(definition.InputExtensions) > 0 && !containsExtension(definition.InputExtensions, inputExtension) {
		return "", fmt.Errorf("selected file extension %q is not supported", inputExtension)
	}

	workingPath, err := copyToProcessingFile(sourcePath)
	if err != nil {
		return "", err
	}
	defer os.Remove(workingPath)
	if err := r.executeProcessorsAtOffset(workingPath, []shared.DownloadStep{processor}, 0, false); err != nil {
		return "", err
	}

	outputExtension := definition.OutputExtension
	if outputExtension == "" {
		outputExtension = filepath.Ext(sourcePath)
	}
	base := strings.TrimSuffix(sourcePath, filepath.Ext(sourcePath))
	outputPath, err := naming.ResolveFilenameConflict(base+".decrypted"+outputExtension, "rename")
	if err != nil {
		return "", err
	}
	if err := os.Rename(workingPath, outputPath); err != nil {
		return "", err
	}
	workingPath = ""
	return outputPath, nil
}

func containsExtension(extensions []string, expected string) bool {
	for _, extension := range extensions {
		if strings.EqualFold(extension, expected) {
			return true
		}
	}
	return false
}

func (r *Resource) actionEventsEmit(resourceID, actionID, status, outputPath, message string) {
	if r.emit == nil {
		return
	}
	r.emit("resourceActionProgress", map[string]interface{}{
		"resourceId": resourceID,
		"actionId":   actionID,
		"status":     status,
		"outputPath": outputPath,
		"message":    message,
	})
}
