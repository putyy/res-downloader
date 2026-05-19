package plugins

import (
	"encoding/json"
	"res-downloader/core/shared"
	"testing"
	"time"
)

func newTestQqPlugin(sent chan shared.MediaInfo) *QqPlugin {
	p := &QqPlugin{}
	p.SetBridge(&shared.Bridge{
		GetVersion: func() string {
			return "test"
		},
		GetResType: func(key string) (bool, bool) {
			return true, true
		},
		TypeSuffix: func(mime string) (string, string) {
			return "", ""
		},
		MediaIsMarked: func(key string) bool {
			return false
		},
		MarkMedia: func(key string) {},
		GetConfig: func(key string) interface{} {
			return true
		},
		Send: func(t string, data interface{}) {
			if t == "newResources" {
				sent <- data.(shared.MediaInfo)
			}
		},
	})
	return p
}

func waitMediaInfo(t *testing.T, sent chan shared.MediaInfo) shared.MediaInfo {
	t.Helper()

	select {
	case media := <-sent:
		return media
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for media info")
	}

	return shared.MediaInfo{}
}

func TestQqPluginHandleMediaBuildsSingleImageSetForWechatAlbum(t *testing.T) {
	sent := make(chan shared.MediaInfo, 1)
	plugin := newTestQqPlugin(sent)

	body := map[string]interface{}{
		"mediaType":     float64(2),
		"description":   "图文标题#话题",
		"id":            "object-123",
		"objectNonceId": "nonce-456",
		"createtime":    float64(1778918400),
		"media": []interface{}{
			map[string]interface{}{
				"url":      "https://wx.example.com/first",
				"urlToken": "?token=first",
				"fileSize": float64(1024),
				"thumbUrl": "https://wx.example.com/first-thumb",
				"coverUrl": "https://wx.example.com/first-cover",
			},
			map[string]interface{}{
				"url":      "https://wx.example.com/second",
				"urlToken": "?token=second",
				"fileSize": float64(2048),
			},
		},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}

	plugin.handleMedia(raw)
	media := waitMediaInfo(t, sent)

	if media.Classify != "image_set" {
		t.Fatalf("Classify = %q, want image_set", media.Classify)
	}
	if media.Suffix != "" {
		t.Fatalf("Suffix = %q, want empty suffix for folder resource", media.Suffix)
	}
	if media.OtherData["image_set_count"] != "2" {
		t.Fatalf("image_set_count = %q, want 2", media.OtherData["image_set_count"])
	}
	if media.OtherData["image_set_urls"] == "" {
		t.Fatal("image_set_urls should contain all image URLs")
	}
	if media.CoverUrl != "https://wx.example.com/first-thumb" {
		t.Fatalf("CoverUrl = %q, want first thumb URL", media.CoverUrl)
	}
}

func TestQqPluginHandleMediaExtractsImageSetAudio(t *testing.T) {
	sent := make(chan shared.MediaInfo, 1)
	plugin := newTestQqPlugin(sent)

	body := map[string]interface{}{
		"mediaType":   float64(2),
		"description": "图文标题",
		"media": []interface{}{
			map[string]interface{}{
				"url":      "https://wx.example.com/first",
				"urlToken": "?token=first",
			},
		},
		"music": map[string]interface{}{
			"url": "https://wx.example.com/bgm.m4a?token=music",
		},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}

	plugin.handleMedia(raw)
	media := waitMediaInfo(t, sent)

	if media.OtherData["image_set_audio_url"] != "https://wx.example.com/bgm.m4a?token=music" {
		t.Fatalf("image_set_audio_url = %q, want audio url", media.OtherData["image_set_audio_url"])
	}
	if media.OtherData["image_set_audio_file_name"] != "bgm.m4a" {
		t.Fatalf("image_set_audio_file_name = %q, want bgm.m4a", media.OtherData["image_set_audio_file_name"])
	}
}

func TestQqPluginHandleMediaKeepsWechatVideoAsVideo(t *testing.T) {
	sent := make(chan shared.MediaInfo, 1)
	plugin := newTestQqPlugin(sent)

	body := map[string]interface{}{
		"mediaType":   float64(4),
		"description": "视频标题",
		"media": []interface{}{
			map[string]interface{}{
				"url":       "https://finder.video.qq.com/video.mp4",
				"urlToken":  "?token=video",
				"decodeKey": "decode-key",
				"fileSize":  float64(4096),
			},
		},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}

	plugin.handleMedia(raw)
	media := waitMediaInfo(t, sent)

	if media.Classify != "video" {
		t.Fatalf("Classify = %q, want video", media.Classify)
	}
	if media.Url != "https://finder.video.qq.com/video.mp4?token=video" {
		t.Fatalf("Url = %q, want full video URL", media.Url)
	}
	if media.DecodeKey != "decode-key" {
		t.Fatalf("DecodeKey = %q, want decode-key", media.DecodeKey)
	}
}
