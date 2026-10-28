package httpapi

import (
	"encoding/json"
	"net/http"
)

func (h *Server) updateApp(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Action string `json:"action"`
		Direct bool   `json:"direct"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.error(w, "invalid update request")
		return
	}
	if h.host.Update == nil {
		h.error(w, "updater unavailable")
		return
	}
	result, err := h.host.Update(r.Context(), input.Action, input.Direct)
	if err != nil {
		h.error(w, err.Error())
		return
	}
	h.success(w, result)
}
