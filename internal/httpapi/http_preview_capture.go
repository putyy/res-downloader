package httpapi

import (
	"io"
	"net/http"
	"os"
	shared "res-downloader/internal/model"
)

func (h *Server) serveCapturePreview(w http.ResponseWriter, r *http.Request, candidate shared.ResourceCandidate, input shared.DownloadInput, processors []shared.DownloadStep) {
	reader, modified, err := h.resources.OpenCapturePreview(input.CaptureKey)
	if err != nil {
		http.Error(w, "Capture is unavailable; reopen the resource and retry", http.StatusConflict)
		return
	}
	defer reader.Close()
	// Do not sniff cached HTML/SVG into an active document. The declared renderer
	// selects the display; unknown binary content remains an attachment-like MIME.
	w.Header().Set("Content-Type", "application/octet-stream")
	for _, track := range candidate.Tracks {
		if track.ID == input.ID && track.MIME != "" {
			w.Header().Set("Content-Type", track.MIME)
			break
		}
	}
	applyDeclaredPreviewMIME(w.Header(), candidate)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "sandbox")
	w.Header().Set("Cache-Control", "no-store")
	if len(processors) == 0 {
		http.ServeContent(w, r, "", modified, reader)
		return
	}
	// Variable-length transforms cannot safely map a requested output range to
	// input offsets. Materialise a bounded complete representation, then serve
	// ranges from that representation, including HEAD and suffix requests.
	size, err := reader.Seek(0, io.SeekEnd)
	if err != nil || size > processedPreviewChunkSize {
		http.Error(w, "Processed capture preview exceeds the 4 MiB input limit; download the resource instead", http.StatusUnprocessableEntity)
		return
	}
	if _, err = reader.Seek(0, io.SeekStart); err != nil {
		http.Error(w, "Failed to read capture", http.StatusInternalServerError)
		return
	}
	temp, err := os.CreateTemp("", ".res-downloader-preview-*")
	if err != nil {
		http.Error(w, "Failed to prepare preview", http.StatusInternalServerError)
		return
	}
	path := temp.Name()
	defer os.Remove(path)
	_, copyErr := io.Copy(temp, reader)
	closeErr := temp.Close()
	if copyErr != nil || closeErr != nil {
		http.Error(w, "Failed to read capture", http.StatusInternalServerError)
		return
	}
	if err := h.resources.ProcessDownload(path, processors, 0, false); err != nil {
		http.Error(w, "Failed to process preview", http.StatusBadGateway)
		return
	}
	processed, err := os.Open(path)
	if err != nil {
		http.Error(w, "Failed to open preview", http.StatusInternalServerError)
		return
	}
	defer processed.Close()
	http.ServeContent(w, r, "", modified, processed)
}
