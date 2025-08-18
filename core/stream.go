package core

import (
	"bufio"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"res-downloader/core/shared"
	"strconv"
	"strings"
)

// M3U8 represents a m3u8 playlist
type M3U8 struct {
	Segments []*Segment
	Keys     map[int]*Key
}

// Segment represents a media segment in a m3u8 playlist
type Segment struct {
	URI      string
	Duration float64
	KeyIndex int
}

// Key represents an encryption key in a m3u8 playlist
type Key struct {
	URI        string
	Method     string
	IV         []byte
	Value      []byte
	IsAbsolute bool
}

// StreamDownloader handles the download and processing of streaming media
type StreamDownloader struct {
	MediaInfo        shared.MediaInfo
	PlaylistURL      *url.URL
	Playlist         *M3U8
	HttpClient       *http.Client
	ProgressCallback func(totalDownloaded, totalSize float64, taskID int, taskProgress float64)
}

// NewStreamDownloader creates a new stream downloader
func NewStreamDownloader(mediaInfo shared.MediaInfo) (*StreamDownloader, error) {
	playlistURL, err := url.Parse(mediaInfo.Url)
	if err != nil {
		return nil, fmt.Errorf("invalid playlist url: %v", err)
	}

	sd := &StreamDownloader{
		MediaInfo:   mediaInfo,
		PlaylistURL: playlistURL,
		HttpClient: &http.Client{},
	}
	return sd, nil
}

// Start begins the download process
func (sd *StreamDownloader) Start() error {
	resp, err := sd.HttpClient.Get(sd.MediaInfo.Url)
	if err != nil {
		return fmt.Errorf("failed to fetch playlist: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read playlist body: %v", err)
	}

	if strings.HasSuffix(sd.MediaInfo.Suffix, ".m3u8") {
		if err := sd.parseM3U8(body); err != nil {
			return err
		}
	} else if strings.HasSuffix(sd.MediaInfo.Suffix, ".mpd") {
		// In a future step, we would call parseMPD here.
		return fmt.Errorf("MPD parsing not yet supported")
	} else {
		return fmt.Errorf("unsupported stream format: %s", sd.MediaInfo.Suffix)
	}

	// Download keys first
	for _, key := range sd.Playlist.Keys {
		keyResp, err := sd.HttpClient.Get(key.URI)
		if err != nil {
			return fmt.Errorf("failed to download key %s: %v", key.URI, err)
		}
		key.Value, err = io.ReadAll(keyResp.Body)
		keyResp.Body.Close()
		if err != nil {
			return fmt.Errorf("failed to read key body %s: %v", key.URI, err)
		}
	}

	var segmentsData [][]byte
	totalSegments := len(sd.Playlist.Segments)
	for i, segment := range sd.Playlist.Segments {
		data, err := sd.downloadSegment(segment, nil) // Headers can be added here if needed
		if err != nil {
			// Log error and continue if possible
			fmt.Printf("Error downloading segment %s: %v\n", segment.URI, err)
			continue
		}

		if segment.KeyIndex > 0 {
			if key, ok := sd.Playlist.Keys[segment.KeyIndex]; ok {
				data, err = sd.decryptSegment(data, key)
				if err != nil {
					fmt.Printf("Error decrypting segment %s: %v\n", segment.URI, err)
					continue
				}
			}
		}
		segmentsData = append(segmentsData, data)
		if sd.ProgressCallback != nil {
			progress := float64(i+1) / float64(totalSegments)
			sd.ProgressCallback(float64(i+1), float64(totalSegments), 0, progress)
		}
	}

	return sd.mergeSegments(segmentsData, sd.MediaInfo.SavePath)
}

// parseM3U8 parses the M3U8 playlist
func (sd *StreamDownloader) parseM3U8(body []byte) error {
	scanner := bufio.NewScanner(bytes.NewReader(body))
	m3u8 := &M3U8{
		Segments: []*Segment{},
		Keys:     make(map[int]*Key),
	}
	var currentSegment *Segment
	keyIndex := 0

	reKey := regexp.MustCompile(`([A-Z-]+)=([^,]+)`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "#EXTINF:") {
			durationStr := strings.TrimPrefix(line, "#EXTINF:")
			parts := strings.Split(durationStr, ",")
			duration, _ := strconv.ParseFloat(parts[0], 64)
			currentSegment = &Segment{Duration: duration, KeyIndex: keyIndex}
		} else if strings.HasPrefix(line, "#EXT-X-KEY:") {
			key := &Key{}
			matches := reKey.FindAllStringSubmatch(strings.TrimPrefix(line, "#EXT-X-KEY:"), -1)
			for _, match := range matches {
				keyName := match[1]
				keyValue := strings.Trim(match[2], `"`)
				switch keyName {
				case "METHOD":
					key.Method = keyValue
				case "URI":
					keyURI, err := sd.PlaylistURL.Parse(keyValue)
					if err == nil {
						key.URI = keyURI.String()
					}
				case "IV":
					ivBytes, err := hex.DecodeString(strings.TrimPrefix(keyValue, "0x"))
					if err == nil {
						key.IV = ivBytes
					}
				}
			}
			keyIndex++
			m3u8.Keys[keyIndex] = key
		} else if !strings.HasPrefix(line, "#") {
			if currentSegment != nil {
				segmentURI, err := sd.PlaylistURL.Parse(line)
				if err != nil {
					continue
				}
				currentSegment.URI = segmentURI.String()
				m3u8.Segments = append(m3u8.Segments, currentSegment)
				currentSegment = nil
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	sd.Playlist = m3u8
	return nil
}

// parseMPD parses the MPD manifest and converts it to an M3U8 structure
func (sd *StreamDownloader) parseMPD(body []byte) error {
	// For simplicity, we'll focus on M3U8 first.
	// A real implementation would parse XML and construct an M3U8 object.
	return fmt.Errorf("MPD parsing not implemented")
}

// downloadSegment downloads a single media segment
func (sd *StreamDownloader) downloadSegment(segment *Segment, headers map[string]string) ([]byte, error) {
	req, err := http.NewRequest("GET", segment.URI, nil)
	if err != nil {
		return nil, err
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := sd.HttpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status: %s", resp.Status)
	}

	return io.ReadAll(resp.Body)
}

// decryptSegment decrypts an encrypted media segment
func (sd *StreamDownloader) decryptSegment(data []byte, key *Key) ([]byte, error) {
	block, err := aes.NewCipher(key.Value)
	if err != nil {
		return nil, err
	}
	iv := key.IV
	if iv == nil {
		iv = make([]byte, 16)
	}
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(data, data)

	// Remove padding
	padding := int(data[len(data)-1])
	return data[:len(data)-padding], nil
}

// mergeSegments merges all downloaded segments into a single file
func (sd *StreamDownloader) mergeSegments(segmentsData [][]byte, outputPath string) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	for _, data := range segmentsData {
		_, err := file.Write(data)
		if err != nil {
			return err
		}
	}
	return nil
}
