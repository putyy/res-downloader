package core

import (
	"encoding/json"
	"res-downloader/core/shared"
	"testing"
)

func TestParseImageSetURLs(t *testing.T) {
	rawURLs, _ := json.Marshal([]string{
		"https://wx.example.com/001.jpg",
		"https://wx.example.com/002.jpg",
	})
	mediaInfo := shared.MediaInfo{
		OtherData: map[string]string{
			"image_set_urls": string(rawURLs),
		},
	}

	urls, err := parseImageSetURLs(mediaInfo)
	if err != nil {
		t.Fatalf("parseImageSetURLs returned error: %v", err)
	}
	if len(urls) != 2 {
		t.Fatalf("len(urls) = %d, want 2", len(urls))
	}
	if urls[1] != "https://wx.example.com/002.jpg" {
		t.Fatalf("urls[1] = %q", urls[1])
	}
}

func TestBuildImageSetMetadata(t *testing.T) {
	rawURLs, _ := json.Marshal([]string{"https://wx.example.com/001.jpg"})
	mediaInfo := shared.MediaInfo{
		Url:         "https://wx.example.com/001.jpg",
		UrlSign:     "sign-123",
		Description: "图文标题#话题",
		OtherData: map[string]string{
			"image_set_urls":         string(rawURLs),
			"image_set_description":  "图文描述",
			"image_set_topic":        "话题",
			"image_set_original_id":  "object-123",
			"image_set_publish_time": "1778918400",
			"image_set_captured_at":  "2026-05-16T00:00:00Z",
			"image_set_audio_url":    "https://wx.example.com/bgm.m4a",
			"image_set_audio_name":   "背景音乐",
		},
	}
	files := []ImageSetFile{
		{Index: 1, FileName: "001.jpg", URL: "https://wx.example.com/001.jpg", Size: 12},
	}
	audio := &ImageSetAudioFile{
		FileName: "bgm.m4a",
		URL:      "https://wx.example.com/bgm.m4a",
		Size:     34,
	}

	metadata := buildImageSetMetadata(mediaInfo, files, audio)

	if metadata.Type != "wechat_channels_image_set" {
		t.Fatalf("Type = %q", metadata.Type)
	}
	if metadata.Cover != "images/001.jpg" {
		t.Fatalf("Cover = %q", metadata.Cover)
	}
	if metadata.ImageCount != 1 {
		t.Fatalf("ImageCount = %d", metadata.ImageCount)
	}
	if metadata.Images[0].FileName != "001.jpg" {
		t.Fatalf("Images[0].FileName = %q", metadata.Images[0].FileName)
	}
	if metadata.Audio == nil {
		t.Fatal("Audio should be present")
	}
	if metadata.Audio.FileName != "bgm.m4a" {
		t.Fatalf("Audio.FileName = %q", metadata.Audio.FileName)
	}
	if metadata.Audio.Path != "audio/bgm.m4a" {
		t.Fatalf("Audio.Path = %q", metadata.Audio.Path)
	}
}
