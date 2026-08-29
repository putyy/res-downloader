package download

import (
	"res-downloader/internal/config"
)

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	shared "res-downloader/internal/model"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	maxHLSPlaylistSize = 4 * 1024 * 1024
	maxHLSSegmentSize  = 128 * 1024 * 1024
	maxHLSSegments     = 100000
	maxHLSTotalSize    = 20 * 1024 * 1024 * 1024
)

type hlsVariant struct {
	url        string
	bandwidth  int64
	resolution string
}

type hlsEncryption struct {
	method string
	keyURL string
	iv     []byte
}

type hlsPart struct {
	url        string
	rangeStart int64
	rangeEnd   int64
	sequence   uint64
	encryption hlsEncryption
}

func (r *PlanRunner) acquireHLS(input shared.DownloadInput, directory string, workers int) (string, error) {
	playlistURL := input.URL
	var playlist []byte
	var err error
	for depth := 0; depth < 3; depth++ {
		playlist, playlistURL, err = fetchHLSResource(r.ctx, r.config, playlistURL, input.Headers, "", maxHLSPlaylistSize)
		if err != nil {
			return "", err
		}
		variants, err := parseHLSVariants(playlist, playlistURL)
		if err != nil {
			return "", err
		}
		if len(variants) == 0 {
			break
		}
		playlistURL = selectHLSVariant(variants, input.Options).url
	}
	parts, endList, err := parseHLSMediaPlaylist(playlist, playlistURL)
	if err != nil {
		return "", err
	}
	if required, _ := input.Options["requireEndList"].(bool); required && !endList {
		return "", errors.New("HLS playlist is live; requireEndList rejected a snapshot download")
	}
	if len(parts) == 0 {
		return "", errors.New("HLS playlist contains no downloadable segments")
	}
	if workers <= 0 {
		workers = 4
	}
	if workers > 32 {
		workers = 32
	}
	if workers > len(parts) {
		workers = len(parts)
	}

	ctx, cancel := context.WithCancel(r.ctx)
	defer cancel()
	paths := make([]string, len(parts))
	defer func() {
		for _, path := range paths {
			if path != "" {
				_ = os.Remove(path)
			}
		}
	}()
	jobs := make(chan int)
	errCh := make(chan error, 1)
	var wait sync.WaitGroup
	var keyMu sync.Mutex
	keyCache := make(map[string][]byte)
	var progressMu sync.Mutex
	var downloaded int64

	worker := func() {
		defer wait.Done()
		for index := range jobs {
			part := parts[index]
			rangeHeader := ""
			if part.rangeEnd >= part.rangeStart && part.rangeStart >= 0 {
				rangeHeader = fmt.Sprintf("bytes=%d-%d", part.rangeStart, part.rangeEnd)
			}
			var partPath string
			var partSize int64
			var fetchErr error
			for attempt := 0; attempt < 3; attempt++ {
				partPath, partSize, fetchErr = fetchHLSSegmentToFile(ctx, r.config, part.url, input.Headers, rangeHeader, directory)
				if fetchErr == nil {
					break
				}
			}
			if fetchErr == nil && part.encryption.method != "" && part.encryption.method != "NONE" {
				if part.encryption.method != "AES-128" {
					fetchErr = fmt.Errorf("unsupported HLS encryption method %q", part.encryption.method)
				} else {
					keyMu.Lock()
					key := append([]byte(nil), keyCache[part.encryption.keyURL]...)
					keyMu.Unlock()
					if len(key) == 0 {
						key, _, fetchErr = fetchHLSResource(ctx, r.config, part.encryption.keyURL, input.Headers, "", 1024)
						if fetchErr == nil {
							keyMu.Lock()
							keyCache[part.encryption.keyURL] = append([]byte(nil), key...)
							keyMu.Unlock()
						}
					}
					if fetchErr == nil {
						fetchErr = decryptHLSSegmentFile(partPath, key, part.encryption.iv, part.sequence)
					}
				}
			}
			if fetchErr != nil {
				if partPath != "" {
					_ = os.Remove(partPath)
				}
				select {
				case errCh <- fmt.Errorf("segment %d: %w", index, fetchErr):
					cancel()
				default:
				}
				return
			}
			paths[index] = partPath
			progressMu.Lock()
			downloaded += partSize
			current := downloaded
			progressMu.Unlock()
			if current > maxHLSTotalSize {
				select {
				case errCh <- errors.New("HLS download exceeds the 20 GiB safety limit"):
					cancel()
				default:
				}
				return
			}
			r.reportProgress(input.ID, float64(current), -1)
		}
	}
	wait.Add(workers)
	for index := 0; index < workers; index++ {
		go worker()
	}
	go func() {
		defer close(jobs)
		for index := range parts {
			select {
			case jobs <- index:
			case <-ctx.Done():
				return
			}
		}
	}()
	wait.Wait()
	select {
	case err := <-errCh:
		return "", err
	default:
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return concatDownloadInputs(directory, paths)
}

func fetchHLSSegmentToFile(ctx context.Context, config *config.Config, resourceURL string, headers map[string]string, rangeHeader, directory string) (string, int64, error) {
	requestCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, resourceURL, nil)
	if err != nil {
		return "", 0, err
	}
	for key, value := range headers {
		if _, forbidden := forbiddenDownloadHeaders[strings.ToLower(key)]; !forbidden {
			request.Header.Set(key, value)
		}
	}
	if request.Header.Get("User-Agent") == "" {
		request.Header.Set("User-Agent", config.UserAgent)
	}
	if rangeHeader != "" {
		request.Header.Set("Range", rangeHeader)
	}
	downloadClient := &FileDownloader{}
	if config.DownloadProxy && config.UpstreamProxy != "" && !strings.Contains(config.UpstreamProxy, config.Port) {
		if proxyURL, parseErr := url.Parse(config.UpstreamProxy); parseErr == nil {
			downloadClient.ProxyUrl = proxyURL
		}
	}
	client := downloadClient.buildClient()
	response, err := client.Do(request)
	if err != nil {
		return "", 0, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", 0, fmt.Errorf("unexpected HLS segment status %d", response.StatusCode)
	}
	temp, err := os.CreateTemp(directory, ".res-downloader-hls-part-*")
	if err != nil {
		return "", 0, err
	}
	path := temp.Name()
	written, copyErr := io.Copy(temp, io.LimitReader(response.Body, maxHLSSegmentSize+1))
	closeErr := temp.Close()
	if copyErr != nil || closeErr != nil || written > maxHLSSegmentSize {
		_ = os.Remove(path)
		if copyErr != nil {
			return "", 0, copyErr
		}
		if closeErr != nil {
			return "", 0, closeErr
		}
		return "", 0, errors.New("HLS segment exceeds the 128 MiB safety limit")
	}
	return path, written, nil
}

func decryptHLSSegmentFile(path string, key, configuredIV []byte, sequence uint64) error {
	if len(key) != aes.BlockSize {
		return fmt.Errorf("HLS AES-128 key is %d bytes, expected 16", len(key))
	}
	file, err := os.OpenFile(path, os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if info.Size()%aes.BlockSize != 0 {
		return errors.New("encrypted HLS segment is not block aligned")
	}
	iv := append([]byte(nil), configuredIV...)
	if len(iv) == 0 {
		iv = make([]byte, aes.BlockSize)
		for index := aes.BlockSize - 1; sequence > 0 && index >= 0; index-- {
			iv[index], sequence = byte(sequence), sequence>>8
		}
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	mode := cipher.NewCBCDecrypter(block, iv)
	buffer := make([]byte, 256*1024)
	var offset int64
	for offset < info.Size() {
		want := int64(len(buffer))
		if remaining := info.Size() - offset; remaining < want {
			want = remaining
		}
		chunk := buffer[:want]
		if _, err := file.ReadAt(chunk, offset); err != nil {
			return err
		}
		mode.CryptBlocks(chunk, chunk)
		if _, err := file.WriteAt(chunk, offset); err != nil {
			return err
		}
		offset += want
	}
	last := make([]byte, aes.BlockSize)
	if _, err := file.ReadAt(last, info.Size()-aes.BlockSize); err != nil {
		return err
	}
	padding := int(last[len(last)-1])
	if padding > 0 && padding <= aes.BlockSize && bytes.Equal(last[len(last)-padding:], bytes.Repeat([]byte{byte(padding)}, padding)) {
		return file.Truncate(info.Size() - int64(padding))
	}
	return nil
}

func fetchHLSResource(ctx context.Context, config *config.Config, resourceURL string, headers map[string]string, rangeHeader string, limit int64) ([]byte, string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, resourceURL, nil)
	if err != nil {
		return nil, resourceURL, err
	}
	for key, value := range headers {
		if _, forbidden := forbiddenDownloadHeaders[strings.ToLower(key)]; !forbidden {
			request.Header.Set(key, value)
		}
	}
	if request.Header.Get("User-Agent") == "" {
		request.Header.Set("User-Agent", config.UserAgent)
	}
	if rangeHeader != "" {
		request.Header.Set("Range", rangeHeader)
	}
	downloadClient := &FileDownloader{}
	if config.DownloadProxy && config.UpstreamProxy != "" && !strings.Contains(config.UpstreamProxy, config.Port) {
		if proxyURL, parseErr := url.Parse(config.UpstreamProxy); parseErr == nil {
			downloadClient.ProxyUrl = proxyURL
		}
	}
	client := downloadClient.buildClient()
	response, err := client.Do(request)
	if err != nil {
		return nil, resourceURL, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusPartialContent {
		return nil, resourceURL, fmt.Errorf("HTTP status %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return nil, resourceURL, err
	}
	if int64(len(data)) > limit {
		return nil, resourceURL, fmt.Errorf("response exceeds %d bytes", limit)
	}
	finalURL := resourceURL
	if response.Request != nil && response.Request.URL != nil {
		finalURL = response.Request.URL.String()
	}
	return data, finalURL, nil
}

func parseHLSVariants(playlist []byte, playlistURL string) ([]hlsVariant, error) {
	lines := hlsLines(playlist)
	variants := make([]hlsVariant, 0)
	for index, line := range lines {
		if !strings.HasPrefix(line, "#EXT-X-STREAM-INF:") {
			continue
		}
		attributes := parseHLSAttributes(strings.TrimPrefix(line, "#EXT-X-STREAM-INF:"))
		uri := nextHLSURI(lines, index+1)
		if uri == "" {
			return nil, errors.New("HLS variant is missing its URI")
		}
		resolved, err := resolveHLSURL(playlistURL, uri)
		if err != nil {
			return nil, err
		}
		bandwidth, _ := strconv.ParseInt(attributes["BANDWIDTH"], 10, 64)
		variants = append(variants, hlsVariant{url: resolved, bandwidth: bandwidth, resolution: attributes["RESOLUTION"]})
	}
	return variants, nil
}

func selectHLSVariant(variants []hlsVariant, options map[string]interface{}) hlsVariant {
	sorted := append([]hlsVariant(nil), variants...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].bandwidth < sorted[j].bandwidth })
	if value, _ := options["variant"].(string); strings.EqualFold(value, "lowest") {
		return sorted[0]
	}
	if raw, exists := options["maxBandwidth"]; exists {
		var maximum int64
		switch value := raw.(type) {
		case float64:
			maximum = int64(value)
		case int64:
			maximum = value
		case int:
			maximum = int64(value)
		}
		if maximum > 0 {
			choice := sorted[0]
			for _, variant := range sorted {
				if variant.bandwidth > maximum {
					break
				}
				choice = variant
			}
			return choice
		}
	}
	return sorted[len(sorted)-1]
}

func parseHLSMediaPlaylist(playlist []byte, playlistURL string) ([]hlsPart, bool, error) {
	lines := hlsLines(playlist)
	parts := make([]hlsPart, 0)
	sequence := uint64(0)
	currentKey := hlsEncryption{}
	nextRangeStart, nextRangeEnd := int64(-1), int64(-1)
	previousRangeEnd := int64(-1)
	endList := false
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "#EXT-X-MEDIA-SEQUENCE:"):
			sequence, _ = strconv.ParseUint(strings.TrimSpace(strings.TrimPrefix(line, "#EXT-X-MEDIA-SEQUENCE:")), 10, 64)
		case strings.HasPrefix(line, "#EXT-X-KEY:"):
			attributes := parseHLSAttributes(strings.TrimPrefix(line, "#EXT-X-KEY:"))
			currentKey = hlsEncryption{method: attributes["METHOD"]}
			if currentKey.method != "NONE" {
				var err error
				currentKey.keyURL, err = resolveHLSURL(playlistURL, attributes["URI"])
				if err != nil {
					return nil, false, err
				}
				currentKey.iv, err = parseHLSIV(attributes["IV"])
				if err != nil {
					return nil, false, err
				}
			}
		case strings.HasPrefix(line, "#EXT-X-MAP:"):
			attributes := parseHLSAttributes(strings.TrimPrefix(line, "#EXT-X-MAP:"))
			resolved, err := resolveHLSURL(playlistURL, attributes["URI"])
			if err != nil {
				return nil, false, err
			}
			start, end, err := parseHLSByteRange(attributes["BYTERANGE"], -1)
			if err != nil {
				return nil, false, err
			}
			parts = append(parts, hlsPart{url: resolved, rangeStart: start, rangeEnd: end, sequence: sequence, encryption: cloneHLSEncryption(currentKey)})
		case strings.HasPrefix(line, "#EXT-X-BYTERANGE:"):
			var err error
			nextRangeStart, nextRangeEnd, err = parseHLSByteRange(strings.TrimPrefix(line, "#EXT-X-BYTERANGE:"), previousRangeEnd+1)
			if err != nil {
				return nil, false, err
			}
		case line == "#EXT-X-ENDLIST":
			endList = true
		case line != "" && !strings.HasPrefix(line, "#"):
			resolved, err := resolveHLSURL(playlistURL, line)
			if err != nil {
				return nil, false, err
			}
			parts = append(parts, hlsPart{
				url: resolved, rangeStart: nextRangeStart, rangeEnd: nextRangeEnd,
				sequence: sequence, encryption: cloneHLSEncryption(currentKey),
			})
			if nextRangeEnd >= 0 {
				previousRangeEnd = nextRangeEnd
			}
			nextRangeStart, nextRangeEnd = -1, -1
			sequence++
			if len(parts) > maxHLSSegments {
				return nil, false, fmt.Errorf("HLS playlist exceeds %d segments", maxHLSSegments)
			}
		}
	}
	return parts, endList, nil
}

func parseHLSByteRange(value string, implicitStart int64) (int64, int64, error) {
	value = strings.Trim(strings.TrimSpace(value), "\"")
	if value == "" {
		return -1, -1, nil
	}
	lengthText, offsetText, hasOffset := strings.Cut(value, "@")
	length, err := strconv.ParseInt(lengthText, 10, 64)
	if err != nil || length <= 0 {
		return 0, 0, errors.New("invalid HLS byte range length")
	}
	start := implicitStart
	if hasOffset {
		start, err = strconv.ParseInt(offsetText, 10, 64)
		if err != nil || start < 0 {
			return 0, 0, errors.New("invalid HLS byte range offset")
		}
	}
	if start < 0 {
		return 0, 0, errors.New("HLS byte range requires an explicit first offset")
	}
	return start, start + length - 1, nil
}

func parseHLSIV(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	value = strings.TrimPrefix(strings.TrimPrefix(value, "0x"), "0X")
	if len(value)%2 != 0 {
		value = "0" + value
	}
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) > aes.BlockSize {
		return nil, errors.New("invalid HLS AES-128 IV")
	}
	iv := make([]byte, aes.BlockSize)
	copy(iv[aes.BlockSize-len(decoded):], decoded)
	return iv, nil
}

func decryptHLSSegment(data, key, configuredIV []byte, sequence uint64) ([]byte, error) {
	if len(key) != 16 {
		return nil, fmt.Errorf("HLS AES-128 key is %d bytes, expected 16", len(key))
	}
	if len(data)%aes.BlockSize != 0 {
		return nil, errors.New("encrypted HLS segment is not block aligned")
	}
	iv := append([]byte(nil), configuredIV...)
	if len(iv) == 0 {
		iv = make([]byte, aes.BlockSize)
		for index := aes.BlockSize - 1; sequence > 0 && index >= 0; index-- {
			iv[index] = byte(sequence)
			sequence >>= 8
		}
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	plain := append([]byte(nil), data...)
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plain, plain)
	if len(plain) > 0 {
		padding := int(plain[len(plain)-1])
		if padding > 0 && padding <= aes.BlockSize && padding <= len(plain) && bytes.Equal(plain[len(plain)-padding:], bytes.Repeat([]byte{byte(padding)}, padding)) {
			plain = plain[:len(plain)-padding]
		}
	}
	return plain, nil
}

func hlsLines(data []byte) []string {
	raw := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		lines = append(lines, strings.TrimSpace(line))
	}
	return lines
}

func nextHLSURI(lines []string, start int) string {
	for index := start; index < len(lines); index++ {
		if lines[index] != "" && !strings.HasPrefix(lines[index], "#") {
			return lines[index]
		}
	}
	return ""
}

func parseHLSAttributes(value string) map[string]string {
	attributes := make(map[string]string)
	for len(value) > 0 {
		separator := strings.IndexByte(value, '=')
		if separator < 0 {
			break
		}
		key := strings.TrimSpace(value[:separator])
		value = value[separator+1:]
		var item string
		if strings.HasPrefix(value, "\"") {
			value = value[1:]
			end := strings.IndexByte(value, '"')
			if end < 0 {
				item = value
				value = ""
			} else {
				item = value[:end]
				value = strings.TrimLeft(value[end+1:], ",")
			}
		} else if comma := strings.IndexByte(value, ','); comma >= 0 {
			item = value[:comma]
			value = value[comma+1:]
		} else {
			item = value
			value = ""
		}
		attributes[key] = strings.TrimSpace(item)
	}
	return attributes
}

func resolveHLSURL(baseURL, reference string) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	ref, err := url.Parse(strings.TrimSpace(reference))
	if err != nil {
		return "", err
	}
	resolved := base.ResolveReference(ref)
	if resolved.Scheme != "http" && resolved.Scheme != "https" {
		return "", errors.New("HLS URI must use HTTP or HTTPS")
	}
	return resolved.String(), nil
}

func cloneHLSEncryption(value hlsEncryption) hlsEncryption {
	value.iv = append([]byte(nil), value.iv...)
	return value
}
