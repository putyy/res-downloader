package proxy

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ResponseDumpConfig enables local HTML/JavaScript diagnostics independently of
// plugin body permissions. It is configured only by development app wiring.
type ResponseDumpConfig struct {
	Directory   string
	Match       func(*http.Request) bool
	ReportError func(error)
}

type responseDumper struct {
	ResponseDumpConfig
	indexMu sync.Mutex
	fileMu  sync.Mutex
}

type responseDumpBody struct {
	File            string `json:"file"`
	Bytes           int    `json:"bytes"`
	StatusCode      int    `json:"statusCode"`
	ContentType     string `json:"contentType"`
	ContentEncoding string `json:"contentEncoding,omitempty"`
	Decoded         bool   `json:"decoded"`
	Truncated       bool   `json:"truncated"`
	DecodeError     string `json:"decodeError,omitempty"`
}

type responseDumpEntry struct {
	Time      string           `json:"time"`
	URL       string           `json:"url"`
	Method    string           `json:"method"`
	Metadata  string           `json:"metadata"`
	Original  responseDumpBody `json:"original"`
	Processed responseDumpBody `json:"processed"`
}

// ConfigureResponseDump must be called before the proxy starts handling traffic.
func (p *Engine) ConfigureResponseDump(config ResponseDumpConfig) {
	if config.Directory != "" && config.Match != nil {
		p.responseDump = &responseDumper{ResponseDumpConfig: config}
	}
}

func (d *responseDumper) prepareRequest(request *http.Request) {
	if d == nil || !d.Match(request) {
		return
	}
	// Prefer inspectable bytes even for scripts outside the plugin's match rules.
	request.Header.Set("Accept-Encoding", "identity")
	request.Header.Del("If-None-Match")
	request.Header.Del("If-Modified-Since")
}

func (d *responseDumper) begin(response *http.Response) func(*http.Response) {
	if d == nil || !d.Match(response.Request) || response.Body == nil {
		return nil
	}
	extension := responseDumpExtension(response)
	if extension == "" || response.Request.Method == http.MethodHead ||
		response.StatusCode == http.StatusNoContent || response.StatusCode == http.StatusNotModified {
		return nil
	}
	now := time.Now().UTC()
	// Only the origin and path are indexed. Never persist request/response
	// headers, URL credentials, fragments or query values in diagnostic metadata.
	address := response.Request.URL
	publicURL := url.URL{Scheme: address.Scheme, Host: address.Host, Path: address.Path, RawPath: address.RawPath}
	entry := responseDumpEntry{
		Time: now.Format(time.RFC3339Nano), URL: publicURL.String(), Method: response.Request.Method,
	}
	resourcePath := responseDumpPath(address, extension)
	var err error
	entry.Original, err = d.saveBody(response, filepath.Join("original", resourcePath))
	if err != nil {
		d.report(err)
		return nil
	}
	return func(processed *http.Response) {
		var err error
		entry.Processed, err = d.saveBody(processed, filepath.Join("processed", resourcePath))
		if err != nil {
			d.report(err)
			return
		}
		if err := d.saveMetadata(&entry, now); err != nil {
			d.report(err)
			return
		}
		d.report(d.appendIndex(entry))
	}
}

func responseDumpExtension(response *http.Response) string {
	contentType, _, _ := mime.ParseMediaType(response.Header.Get("Content-Type"))
	switch strings.ToLower(contentType) {
	case "text/html", "application/xhtml+xml":
		return ".html"
	case "application/javascript", "text/javascript", "application/x-javascript", "text/ecmascript", "application/ecmascript":
		return ".js"
	}
	switch strings.ToLower(filepath.Ext(response.Request.URL.Path)) {
	case ".html", ".htm":
		return ".html"
	case ".js", ".mjs", ".cjs":
		return ".js"
	}
	return ""
}

func (d *responseDumper) saveBody(response *http.Response, name string) (responseDumpBody, error) {
	// Bound both encoded and decoded snapshots without consuming the client's
	// stream. Larger/erroring reads are explicitly marked as incomplete.
	const limit = 32 << 20
	body, truncated := readBodyPrefix(&response.Body, limit)
	encoding := strings.ToLower(strings.TrimSpace(response.Header.Get("Content-Encoding")))
	snapshot := responseDumpBody{
		StatusCode: response.StatusCode, ContentType: response.Header.Get("Content-Type"),
		ContentEncoding: encoding, Truncated: truncated, Decoded: true,
	}
	var reader io.ReadCloser
	var err error
	switch encoding {
	case "", "identity":
	case "gzip":
		reader, err = gzip.NewReader(bytes.NewReader(body))
	case "deflate":
		reader, err = zlib.NewReader(bytes.NewReader(body))
	default:
		err = fmt.Errorf("unsupported content encoding")
	}
	if reader != nil {
		var decoded []byte
		decoded, err = io.ReadAll(io.LimitReader(reader, limit+1))
		_ = reader.Close()
		if err == nil {
			if len(decoded) > limit {
				snapshot.Truncated = true
				decoded = decoded[:limit]
			}
			body = decoded
		}
	}
	if err != nil {
		snapshot.Decoded = false
		snapshot.DecodeError = err.Error()
		name += ".encoded"
	}
	snapshot.Bytes = len(body)
	snapshot.File, err = d.saveFile(name, body)
	return snapshot, err
}

// Preserve the site's directory and file names, without allowing URL paths to
// escape the dump directory or introduce path separators inside a segment.
func responseDumpPath(address *url.URL, extension string) string {
	host := strings.ToLower(address.Hostname())
	if port := address.Port(); port != "" {
		host += "_" + port
	}
	parts := []string{responseDumpPathSegment(host)}
	for _, segment := range strings.Split(address.EscapedPath(), "/") {
		if segment != "" {
			parts = append(parts, responseDumpPathSegment(segment))
		}
	}
	if len(parts) == 1 || strings.HasSuffix(address.EscapedPath(), "/") {
		parts = append(parts, "index"+extension)
	} else {
		suffix := strings.ToLower(filepath.Ext(parts[len(parts)-1]))
		isHTML := extension == ".html" && (suffix == ".html" || suffix == ".htm")
		isJS := extension == ".js" && (suffix == ".js" || suffix == ".mjs" || suffix == ".cjs")
		if !isHTML && !isJS {
			parts[len(parts)-1] += extension
		}
	}
	return filepath.Join(parts...)
}

func responseDumpPathSegment(value string) string {
	var safe strings.Builder
	for _, character := range []byte(value) {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' || strings.ContainsRune("-_.%", rune(character)) {
			safe.WriteByte(character)
		} else {
			fmt.Fprintf(&safe, "%%%02X", character)
		}
	}
	segment := safe.String()
	if segment == "" || segment == "." || segment == ".." {
		segment = "_" + strings.ReplaceAll(segment, ".", "%2E")
	}
	// Leave space for a variant hash and extension under common filename limits.
	if len(segment) > 160 {
		segment = fmt.Sprintf("%s-%x", segment[:96], sha256.Sum256([]byte(segment)))
	}
	return segment
}

// Reuse identical bytes. Different bodies at the same URL get a content hash
// before the extension, so repeated captures never overwrite earlier evidence.
func (d *responseDumper) saveFile(name string, body []byte) (string, error) {
	d.fileMu.Lock()
	defer d.fileMu.Unlock()
	digest := fmt.Sprintf("%x", sha256.Sum256(body))
	extension := filepath.Ext(name)
	stem := strings.TrimSuffix(name, extension)
	for _, candidate := range []string{name, stem + "-" + digest[:16] + extension, stem + "-" + digest + extension} {
		path := filepath.Join(d.Directory, candidate)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return "", err
		}
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if errors.Is(err, os.ErrExist) {
			existing, readErr := os.ReadFile(path)
			if readErr != nil {
				return "", readErr
			}
			if bytes.Equal(existing, body) {
				return filepath.ToSlash(candidate), nil
			}
			continue
		}
		if err != nil {
			return "", err
		}
		_, writeErr := file.Write(body)
		closeErr := file.Close()
		if writeErr != nil || closeErr != nil {
			_ = os.Remove(path) // Only remove the incomplete file just created here.
			return "", errors.Join(writeErr, closeErr)
		}
		return filepath.ToSlash(candidate), nil
	}
	return "", errors.New("response dump content hash collision")
}

func (d *responseDumper) saveMetadata(entry *responseDumpEntry, now time.Time) error {
	directory := filepath.Join(d.Directory, "metadata")
	if err := os.MkdirAll(directory, 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(directory, now.Format("20060102-150405.000000000")+"-*.json")
	if err != nil {
		return err
	}
	entry.Metadata = filepath.ToSlash(filepath.Join("metadata", filepath.Base(file.Name())))
	metadata, marshalErr := json.MarshalIndent(entry, "", "  ")
	var writeErr error
	if marshalErr == nil {
		_, writeErr = file.Write(append(metadata, '\n'))
	}
	return errors.Join(marshalErr, writeErr, file.Close())
}

func (d *responseDumper) appendIndex(entry responseDumpEntry) error {
	line, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	d.indexMu.Lock()
	defer d.indexMu.Unlock()
	file, err := os.OpenFile(filepath.Join(d.Directory, "index.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(append(line, '\n'))
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

func (d *responseDumper) report(err error) {
	if err != nil && d.ReportError != nil {
		d.ReportError(err)
	}
}
