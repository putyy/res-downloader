package plugins

import (
	"bytes"
	"encoding/json"
	"net/http"
	"res-downloader/core/shared"
	"testing"
)

// TestExtractFeedObjects 验证从 FinderGetRecommend 响应中提取批量身份
func TestExtractFeedObjects(t *testing.T) {
	imageURL := "https://wx.qlogo.cn/mmhead/example"
	videoURL := "https://finder.video.qq.com/251/20302/stodownload?encfilekey=video"
	data := map[string]interface{}{
		"object": []interface{}{
			map[string]interface{}{
				"id":            "14975140960947210868",
				"objectNonceId": "8887810777705516498_4_140_84_32_x",
				"sessionBuffer": "session-a",
				"objectDesc": map[string]interface{}{
					"media": []interface{}{
						map[string]interface{}{"url": imageURL, "mediaType": float64(9)},
						map[string]interface{}{"url": videoURL, "mediaType": float64(0)},
					},
				},
			},
			map[string]interface{}{
				"id":            float64(1.4974240473550232e+18),
				"objectNonceId": "18160454765900885364_4_0_84_0_y",
				"sessionBuffer": "session-b",
				"objectDesc": map[string]interface{}{
					"media": []interface{}{
						map[string]interface{}{"url": "https://finder.video.qq.com/c/d?encfilekey=2"},
					},
				},
			},
			// 无 objectDesc 的条目应被跳过
			map[string]interface{}{"id": "999"},
		},
	}

	feeds := extractFeedObjects(data)
	if len(feeds) != 2 {
		t.Fatalf("expected 2 feeds, got %d", len(feeds))
	}
	if feeds[0].objectId != "14975140960947210868" || feeds[0].nonceId == "" || feeds[0].sessionBuffer != "session-a" {
		t.Errorf("feed[0] mismatch: %+v", feeds[0])
	}
	wantSign := shared.Md5(videoURL)
	if feeds[0].urlSign != wantSign {
		t.Errorf("feed[0] urlSign mismatch: %q want %q", feeds[0].urlSign, wantSign)
	}
	ordered := feeds[0].objectDesc["media"].([]interface{})
	if ordered[0].(map[string]interface{})["url"] != videoURL {
		t.Fatalf("expected video candidate first, got %+v", ordered)
	}
	if feeds[1].objectId != "1497424047355023200" && len(feeds[1].objectId) < 10 {
		t.Errorf("feed[1] float64 id conversion suspicious: %q", feeds[1].objectId)
	}
}

// TestRecommendReportLearning 验证 type=8 上报后映射可查
func TestRecommendReportLearning(t *testing.T) {
	p := &QqPlugin{}
	p.learnSignMapping("sign-a", commentIdentity{ObjectId: "oid-a", NonceId: "nonce-a", SessionBuffer: "sb-a", Source: "post_recommend"})
	id := p.ResolveIdentity("sign-a")
	if id.ObjectId != "oid-a" || id.NonceId != "nonce-a" || id.SessionBuffer != "sb-a" || !id.isComplete() {
		t.Errorf("identity mismatch: %+v", id)
	}
}

func TestPostRecommendEmitsAuthoritativeResource(t *testing.T) {
	bridge := newMockBridge()
	p := &QqPlugin{}
	p.SetBridge(&bridge.Bridge)
	rawURL := "https://finder.video.qq.com/251/20304/stodownload?encfilekey=feed-key"
	imageURL := "https://wx.qlogo.cn/mmhead/feed-cover"
	urlSign := shared.Md5(rawURL)
	// 真实链路中通用资源层可能先标记相同 URL。权威 feed 资源仍须发出，
	// 否则前端拿不到与 sessionBuffer 同源的可评论资源。
	bridge.marked[urlSign] = true
	payload := map[string]interface{}{
		"name": "FinderGetRecommend",
		"data": map[string]interface{}{
			"object": []interface{}{
				map[string]interface{}{
					"id":            "14975140960947210868",
					"objectNonceId": "8887810777705516498_4_140_84_32_x",
					"sessionBuffer": "session-a",
					"objectDesc": map[string]interface{}{
						"description": "同源推荐视频",
						"media": []interface{}{
							map[string]interface{}{"url": imageURL, "fileSize": float64(512), "mediaType": float64(9)},
							map[string]interface{}{"url": rawURL, "fileSize": float64(1024), "mediaType": float64(0)},
						},
					},
				},
			},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	report := func() {
		req, err := http.NewRequest(http.MethodPost, "https://wxapp.tc.qq.com/res-downloader/wechat?type=9", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		p.handlePostReport(req)
	}
	report()

	if len(bridge.sent) != 1 || bridge.sent[0].Type != "newResources" {
		t.Fatalf("expected authoritative newResources event, got %+v", bridge.sent)
	}
	res, ok := bridge.sent[0].Data.(shared.MediaInfo)
	if !ok {
		t.Fatalf("unexpected resource type: %T", bridge.sent[0].Data)
	}
	if res.UrlSign != urlSign || res.OtherData["wx_identity_source"] != "post_recommend" {
		t.Fatalf("authoritative resource mismatch: %+v", res)
	}
	id := p.ResolveIdentity(res.UrlSign)
	if !id.isComplete() || id.ObjectId != "14975140960947210868" || id.SessionBuffer != "session-a" {
		t.Fatalf("resource identity not learned from same feed: %+v", id)
	}

	// 推荐流可重复上报同一批次；插件内去重应阻止重复前端资源事件。
	report()
	if len(bridge.sent) != 1 {
		t.Fatalf("expected one authoritative resource after duplicate report, got %d", len(bridge.sent))
	}
}

func TestPostRecommendEmitsImageCommentTargetWhenImageCaptureDisabled(t *testing.T) {
	bridge := newMockBridge()
	bridge.resTypes = map[string]bool{"video": true, "image": false, "all": false}
	p := &QqPlugin{}
	p.SetBridge(&bridge.Bridge)
	rawURL := "https://finder.video.qq.com/251/20304/stodownload?encfilekey=cover-key&wxampicformat=hevc"
	payload := map[string]interface{}{
		"name": "FinderGetRecommend",
		"data": map[string]interface{}{
			"object": []interface{}{
				map[string]interface{}{
					"id":            "14975140960947210868",
					"objectNonceId": "8887810777705516498_4_140_84_32_x",
					"sessionBuffer": "session-a",
					"objectDesc": map[string]interface{}{
						"description": "仅下发封面的推荐动态",
						"media": []interface{}{
							map[string]interface{}{"url": rawURL, "fileSize": float64(512), "mediaType": float64(9)},
						},
					},
				},
			},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, "https://wxapp.tc.qq.com/res-downloader/wechat?type=9", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	p.handlePostReport(req)

	if len(bridge.sent) != 1 || bridge.sent[0].Type != "newResources" {
		t.Fatalf("expected image comment target event, got %+v", bridge.sent)
	}
	res := bridge.sent[0].Data.(shared.MediaInfo)
	if res.Classify != "image" || res.OtherData["wx_comment_target"] != "1" || res.OtherData["wx_identity_source"] != "post_recommend" {
		t.Fatalf("unexpected comment target: %+v", res)
	}
	if _, err := p.AddCommentTask(res.UrlSign, res.Id); err != nil {
		t.Fatalf("expected complete target identity: %v", err)
	}
}
