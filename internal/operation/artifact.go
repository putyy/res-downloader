package operation

import (
	"io"
	"mime"
	m "res-downloader/internal/model"
	"strings"
	"unicode/utf8"
)

type ArtifactBackend interface {
	OpenArtifact(m.OperationArtifact) (io.ReadSeekCloser, m.OperationArtifact, error)
}

func (s *Service) Artifact(artifactID string) (m.OperationArtifact, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.available(); err != nil {
		return m.OperationArtifact{}, err
	}
	if err := s.cleanLocked(false, false); err != nil {
		return m.OperationArtifact{}, err
	}
	a, ok := s.artifacts[artifactID]
	if !ok {
		return a, problem("not_found", "artifact not found")
	}
	backend, ok := s.backend.(ArtifactBackend)
	if !ok {
		return a, problem("artifact_unavailable", "artifact storage unavailable")
	}
	reader, current, err := backend.OpenArtifact(a)
	if reader != nil {
		reader.Close()
	}
	if err != nil && (current.Status == "" || current.Status == "available") {
		current.Status = "unavailable"
	}
	return current, nil
}
func (s *Service) ReadText(artifactID string, offset, limit int64) (map[string]interface{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.available(); err != nil {
		return nil, err
	}
	if offset < 0 || limit < 4 || limit > MaxTextBytes || offset >= MaxTextTotalBytes {
		return nil, problem("invalid_range", "text reads require 4–32768 bytes within the first 1 MiB")
	}
	if err := s.cleanLocked(false, false); err != nil {
		return nil, err
	}
	a, ok := s.artifacts[artifactID]
	if !ok {
		return nil, problem("not_found", "artifact not found")
	}
	backend, ok := s.backend.(ArtifactBackend)
	if !ok {
		return nil, problem("artifact_unavailable", "artifact storage unavailable")
	}
	reader, current, err := backend.OpenArtifact(a)
	if err != nil {
		return nil, problem("artifact_unavailable", "artifact is missing or expired")
	}
	defer reader.Close()
	mediaType, _, parseErr := mime.ParseMediaType(current.MIME)
	if parseErr != nil || (!strings.HasPrefix(mediaType, "text/") && mediaType != "application/json" && !strings.HasSuffix(mediaType, "+json")) {
		return nil, problem("not_text", "artifact is not text")
	}
	if current.Size > MaxTextTotalBytes {
		return nil, problem("text_too_large", "text artifact exceeds the 1 MiB read budget")
	}
	if offset > current.Size {
		return nil, problem("invalid_range", "offset exceeds artifact size")
	}
	if _, err = reader.Seek(offset, io.SeekStart); err != nil {
		return nil, err
	}
	buf := make([]byte, min(limit, current.Size-offset))
	n, err := io.ReadFull(reader, buf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return nil, err
	}
	buf = buf[:n]
	if n > 0 && !utf8.RuneStart(buf[0]) {
		return nil, problem("invalid_offset", "offset is not a UTF-8 character boundary")
	}
	validEnd := 0
	for validEnd < len(buf) {
		if !utf8.FullRune(buf[validEnd:]) {
			if offset+int64(n) >= current.Size {
				return nil, problem("invalid_utf8", "artifact ends with incomplete UTF-8")
			}
			break
		}
		runeValue, size := utf8.DecodeRune(buf[validEnd:])
		if runeValue == utf8.RuneError && size == 1 {
			return nil, problem("invalid_utf8", "artifact is not valid UTF-8")
		}
		validEnd += size
	}
	buf = buf[:validEnd]

	next := offset + int64(len(buf))
	return map[string]interface{}{"text": string(buf), "offset": offset, "nextOffset": next, "size": current.Size, "truncated": next < current.Size, "encoding": "utf-8"}, nil
}
