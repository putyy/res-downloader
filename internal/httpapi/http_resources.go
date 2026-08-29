package httpapi

import (
	"encoding/json"
	"net/http"
	shared "res-downloader/internal/model"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (h *Server) filterResources(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Types []string `json:"types"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		h.error(w, err.Error())
		return
	}
	h.resources.SetTypes(data.Types)
	h.success(w)
}

func (h *Server) clearResources(w http.ResponseWriter, _ *http.Request) {
	h.resources.Clear()
	h.success(w)
}

func (h *Server) deleteResources(w http.ResponseWriter, r *http.Request) {
	var data struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		h.error(w, err.Error())
		return
	}
	h.resources.DeleteMany(data.IDs)
	h.success(w)
}

func (h *Server) listResources(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Offset int `json:"offset"`
		Limit  int `json:"limit"`
	}
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			h.error(w, err.Error())
			return
		}
	}
	h.success(w, h.resources.ListPage(data.Offset, data.Limit))
}

func (h *Server) importResources(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Items []shared.ResourceView `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		h.error(w, err.Error())
		return
	}
	items, err := h.resources.Import(data.Items)
	if err != nil {
		h.error(w, err.Error())
		return
	}
	h.success(w, items)
}

func (h *Server) resourceAction(w http.ResponseWriter, r *http.Request) {
	var data struct {
		ID       string `json:"id"`
		ActionID string `json:"actionId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		h.error(w, err.Error())
		return
	}
	candidate, exists := h.resources.Candidate(data.ID)
	if !exists {
		h.error(w, "resource not found")
		return
	}
	definition, processor, err := h.plugins.ResolveFileAction(candidate, data.ActionID)
	if err != nil {
		h.error(w, err.Error())
		return
	}
	patterns := make([]string, 0, len(definition.InputExtensions))
	for _, extension := range definition.InputExtensions {
		patterns = append(patterns, "*"+extension)
	}
	pattern := strings.Join(patterns, ";")
	if pattern == "" {
		pattern = "*"
	}
	filePath, err := runtime.OpenFileDialog(h.host.context(), runtime.OpenDialogOptions{
		Filters: []runtime.FileFilter{{DisplayName: h.pluginActionDisplayName(definition), Pattern: pattern}},
		Title:   h.pluginActionDisplayName(definition),
	})
	if err != nil {
		h.error(w, err.Error())
		return
	}
	if filePath == "" {
		h.success(w, respData{"cancelled": true})
		return
	}
	go h.resources.ProcessFileAction(candidate, data.ActionID, definition, processor, filePath)
	h.success(w, respData{"started": true})
}

func (h *Server) pluginActionDisplayName(definition shared.PluginActionDefinition) string {
	locale := h.config.Snapshot().Locale
	if entry, ok := definition.Locales[locale]; ok && entry.Name != "" {
		return entry.Name
	}
	if index := strings.Index(locale, "-"); index > 0 {
		if entry, ok := definition.Locales[locale[:index]]; ok && entry.Name != "" {
			return entry.Name
		}
	}
	if entry, ok := definition.Locales["en"]; ok && entry.Name != "" {
		return entry.Name
	}
	return "Process Local File"
}
