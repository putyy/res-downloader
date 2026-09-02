package plugins

import (
	"encoding/json"
	"res-downloader/core/shared"
	"strings"
	"testing"
	"time"
)

// mockBridge 捕获插件行为，用于单元测试
type mockBridge struct {
	shared.Bridge
	sent     []sentEvent
	marked   map[string]bool
	resTypes map[string]bool
}

type sentEvent struct {
	Type string
	Data interface{}
}

func newMockBridge() *mockBridge {
	m := &mockBridge{
		marked:   map[string]bool{},
		resTypes: map[string]bool{"all": true},
	}
	m.Bridge = shared.Bridge{
		GetVersion: func() string { return "test" },
		GetResType: func(key string) (bool, bool) {
			v, ok := m.resTypes[key]
			return v, ok
		},
		TypeSuffix: func(mime string) (string, string) { return "", "" },
		MediaIsMarked: func(key string) bool {
			return m.marked[key]
		},
		MarkMedia: func(key string) {
			m.marked[key] = true
		},
		GetConfig: func(key string) interface{} { return nil },
		Send: func(t string, data interface{}) {
			m.sent = append(m.sent, sentEvent{Type: t, Data: data})
		},
		Log: func(format string, v ...interface{}) {},
	}
	return m
}

// 模拟真实 objectDesc（字段名来自线上 wx_debug_keys）
const testObjectDesc = `{
	"description": "测试视频 #话题",
	"shortTitle": "",
	"mediaType": 0,
	"media": [{
		"url": "https://finder.video.qq.com/251/20302/stodownload?encfilekey=abc&token=xyz",
		"urlToken": "&extra=1",
		"fileSize": 1024000,
		"mediaType": 0,
		"decodeKey": "542254831",
		"coverUrl": "https://finder.video.qq.com/cover",
		"spec": [{"fileFormat": "xWT111"}, {"fileFormat": "xWT156"}]
	}]
}`

func TestHandleMediaWrapped(t *testing.T) {
	bridge := newMockBridge()
	p := &QqPlugin{}
	p.SetBridge(&bridge.Bridge)

	body, _ := json.Marshal(map[string]interface{}{
		"od":   json.RawMessage(testObjectDesc),
		"oid":  "14917557575664601299",
		"onid": "7714135876886077727_0_140_2_32_x",
	})
	p.handleMedia(body, "1")           // type=1：扫描值，不应学习映射
	time.Sleep(100 * time.Millisecond) // Send 在 goroutine 中执行

	if len(bridge.sent) != 1 || bridge.sent[0].Type != "newResources" {
		t.Fatalf("expected 1 newResources event, got %+v", bridge.sent)
	}
	res, ok := bridge.sent[0].Data.(shared.MediaInfo)
	if !ok {
		t.Fatalf("event data is not MediaInfo: %T", bridge.sent[0].Data)
	}
	if res.OtherData["wx_object_id"] != "14917557575664601299" {
		t.Errorf("wx_object_id mismatch: %q", res.OtherData["wx_object_id"])
	}
	if res.OtherData["wx_nonce_id"] != "7714135876886077727_0_140_2_32_x" {
		t.Errorf("wx_nonce_id mismatch: %q", res.OtherData["wx_nonce_id"])
	}
	if res.OtherData["wx_channel"] != "1" {
		t.Errorf("wx_channel marker mismatch: %q", res.OtherData["wx_channel"])
	}
	if res.Description != "测试视频 #话题" {
		t.Errorf("description mismatch: %q", res.Description)
	}
	if res.DecodeKey != "542254831" {
		t.Errorf("decodeKey mismatch: %q", res.DecodeKey)
	}

	// urlSign 应为 Md5(未拼接 urlToken 的 url)
	wantSign := shared.Md5("https://finder.video.qq.com/251/20302/stodownload?encfilekey=abc&token=xyz")
	if res.UrlSign != wantSign {
		t.Errorf("urlSign mismatch: %q want %q", res.UrlSign, wantSign)
	}
	// type=1 通道不应学习映射（扫描值不可信）
	if got := p.ResolveIdentity(wantSign); got.ObjectId != "" {
		t.Errorf("type=1 should not learn mapping, got %+v", got)
	}

	// type=2 通道（detail 钩子，同源可信）应学习映射
	bridge2 := newMockBridge()
	p2 := &QqPlugin{}
	p2.SetBridge(&bridge2.Bridge)
	p2.handleMedia(body, "2")
	time.Sleep(100 * time.Millisecond)
	if got := p2.ResolveIdentity(wantSign); got.ObjectId != "14917557575664601299" {
		t.Errorf("type=2 should learn mapping, got %+v", got)
	}
}

func TestHandleMediaFeedModelIdentityLinksActualVideo(t *testing.T) {
	bridge := newMockBridge()
	p := &QqPlugin{}
	p.SetBridge(&bridge.Bridge)

	body, _ := json.Marshal(map[string]interface{}{
		"od":             json.RawMessage(testObjectDesc),
		"oid":            "14917557575664601299",
		"onid":           "7714135876886077727_0_140_2_32_x",
		"sb":             "feed-model-session-buffer",
		"identitySource": "feed_model",
	})
	p.handleMedia(body, "1")

	wantSign := shared.Md5("https://finder.video.qq.com/251/20302/stodownload?encfilekey=abc&token=xyz")
	identity := p.ResolveIdentity(wantSign)
	if !identity.isComplete() || identity.ObjectId != "14917557575664601299" ||
		identity.NonceId != "7714135876886077727_0_140_2_32_x" ||
		identity.SessionBuffer != "feed-model-session-buffer" || identity.Source != "feed_model" {
		t.Fatalf("feed model identity was not linked to actual video: %+v", identity)
	}
	if len(bridge.sent) != 1 || bridge.sent[0].Type != "newResources" {
		t.Fatalf("expected actual video resource event, got %+v", bridge.sent)
	}
	res := bridge.sent[0].Data.(shared.MediaInfo)
	if res.Classify != "video" || res.OtherData["wx_comment_target"] != "1" ||
		res.OtherData["wx_identity_source"] != "feed_model" {
		t.Fatalf("actual video should be a verified comment target: %+v", res)
	}
	if _, err := p.AddCommentTask(res.UrlSign, res.Id); err != nil {
		t.Fatalf("actual video should queue comments without a separate cover target: %v", err)
	}
}

func TestHandleMediaRejectsIncompleteOrUnmarkedTypeOneIdentity(t *testing.T) {
	tests := []map[string]interface{}{
		{
			"od": json.RawMessage(testObjectDesc), "oid": "oid", "onid": "nonce", "sb": "session",
		},
		{
			"od": json.RawMessage(testObjectDesc), "oid": "oid", "onid": "nonce", "identitySource": "feed_model",
		},
	}
	for i, bodyValue := range tests {
		bridge := newMockBridge()
		p := &QqPlugin{}
		p.SetBridge(&bridge.Bridge)
		body, _ := json.Marshal(bodyValue)
		p.handleMedia(body, "1")
		res := bridge.sent[0].Data.(shared.MediaInfo)
		if identity := p.ResolveIdentity(res.UrlSign); identity.isComplete() {
			t.Fatalf("case %d unexpectedly trusted incomplete/unmarked type=1 identity: %+v", i, identity)
		}
	}
}

func TestLearnSignMappingOnlyReportsSemanticChanges(t *testing.T) {
	p := &QqPlugin{}
	identity := commentIdentity{
		ObjectId: "oid", NonceId: "nonce", SessionBuffer: "session", Source: "feed_model",
	}
	if changed := p.learnSignMapping("sign", identity); !changed {
		t.Fatal("first complete identity should be reported as changed")
	}
	if changed := p.learnSignMapping("sign", identity); changed {
		t.Fatal("identical getter reports should not be treated as identity changes")
	}
	identity.SessionBuffer = "refreshed-session"
	if changed := p.learnSignMapping("sign", identity); !changed {
		t.Fatal("refreshed sessionBuffer should be reported as changed")
	}
}

func TestHandleMediaLegacyRawObjectDesc(t *testing.T) {
	bridge := newMockBridge()
	p := &QqPlugin{}
	p.SetBridge(&bridge.Bridge)

	// 旧版注入 JS 直接回传 objectDesc（无包装）
	p.handleMedia([]byte(testObjectDesc), "1")
	time.Sleep(100 * time.Millisecond)
	if len(bridge.sent) != 1 || bridge.sent[0].Type != "newResources" {
		t.Fatalf("legacy objectDesc should still produce resource, got %+v", bridge.sent)
	}
}

func TestIsFinderVideoHost(t *testing.T) {
	accepted := []string{
		"finder.video.qq.com",
		"findera4.video.qq.com:443",
		"FINDERB2.VIDEO.QQ.COM",
	}
	for _, host := range accepted {
		if !isFinderVideoHost(host) {
			t.Errorf("expected finder host %q to be accepted", host)
		}
	}

	rejected := []string{
		"video.qq.com",
		"evilfinder.video.qq.com",
		"finder.video.qq.com.example.com",
	}
	for _, host := range rejected {
		if isFinderVideoHost(host) {
			t.Errorf("expected unrelated host %q to be rejected", host)
		}
	}
}

func TestSafeURLShapeRedactsValues(t *testing.T) {
	shape := safeURLShape("https://findera4.video.qq.com:443/secret/object/path?encfilekey=private-enc&token=private-token&foo=bar")
	for _, secret := range []string{"secret/object/path", "private-enc", "private-token", "foo=bar"} {
		if strings.Contains(shape, secret) {
			t.Fatalf("safeURLShape leaked %q in %q", secret, shape)
		}
	}
	for _, expected := range []string{"host:findera4.video.qq.com", "keys:encfilekey,foo,token", "path:len:", "enc:len:", "token:len:"} {
		if !strings.Contains(shape, expected) {
			t.Fatalf("safeURLShape(%q) missing %q", shape, expected)
		}
	}
}

func TestHandleCommentsExtraction(t *testing.T) {
	bridge := newMockBridge()
	p := &QqPlugin{}
	p.SetBridge(&bridge.Bridge)

	mediaUrl := "https://finder.video.qq.com/251/20302/stodownload?encfilekey=abc&token=xyz"
	payload, _ := json.Marshal(map[string]interface{}{
		"objectId": "14917557575664601299",
		"arg":      "14917557575664601299",
		"data": map[string]interface{}{
			"object": map[string]interface{}{
				"id":            "14917557575664601299",
				"objectNonceId": "7714135876886077727_0_140",
				"objectDesc": map[string]interface{}{
					"media": []interface{}{
						map[string]interface{}{"url": mediaUrl},
					},
				},
			},
			// 真实 FinderGetCommentList 响应结构（字段名来自线上抓包）
			"commentInfo": []interface{}{
				map[string]interface{}{
					"nickname": "用户甲", "content": "说得好", "likeCount": float64(42),
					"createtime": "1785160000", "expandCommentCount": float64(22),
					"ipRegionInfo": map[string]interface{}{"regionText": "云南"},
				},
				map[string]interface{}{"nickname": "用户乙", "content": "学习了"},
			},
		},
	})
	p.handleComments(payload)

	if len(bridge.sent) != 1 || bridge.sent[0].Type != "newComments" {
		t.Fatalf("expected newComments event, got %+v", bridge.sent)
	}
	data := bridge.sent[0].Data.(map[string]interface{})
	if data["objectId"] != "14917557575664601299" {
		t.Errorf("objectId mismatch: %v", data["objectId"])
	}
	wantSign := shared.Md5(mediaUrl)
	if data["urlSign"] != wantSign {
		t.Errorf("urlSign mismatch: %v want %v", data["urlSign"], wantSign)
	}
	comments, ok := data["comments"].([]CommentItem)
	if !ok || len(comments) != 2 {
		t.Fatalf("comments extraction failed: %+v", data["comments"])
	}
	if comments[0].NickName != "用户甲" || comments[0].Content != "说得好" || comments[0].LikeCount != 42 {
		t.Errorf("comment[0] mismatch: %+v", comments[0])
	}
	if comments[0].CreatedAt != 1785160000 {
		t.Errorf("createtime(string) parse failed: %v", comments[0].CreatedAt)
	}
	if comments[0].ReplyCnt != 22 || comments[0].Region != "云南" {
		t.Errorf("replyCnt/region mismatch: %+v", comments[0])
	}
	// 映射也应被学习
	if got := p.ResolveIdentity(wantSign); got.ObjectId != "14917557575664601299" {
		t.Errorf("signMap not learned from comments: %+v", got)
	}
}

func TestCommentTaskLifecycle(t *testing.T) {
	p := &QqPlugin{}
	p.learnSignMapping("sign-1", commentIdentity{ObjectId: "oid-1", NonceId: "nid-1", SessionBuffer: "sb-1", Source: "post_recommend"})
	task, err := p.AddCommentTask("sign-1", "res-1")
	if err != nil {
		t.Fatal(err)
	}
	if task.RequestId == "" || task.UrlSign != "sign-1" {
		t.Fatalf("task metadata missing: %+v", task)
	}
	tasks := p.popCommentTasks()
	if len(tasks) != 1 || tasks[0].ObjectId != "oid-1" {
		t.Fatalf("pop failed: %+v", tasks)
	}
	if got := p.resIdOf("oid-1"); got != "res-1" {
		t.Errorf("resIdOf mismatch: %q", got)
	}
	// 第二次 pop 应为空（任务已被取走）
	if tasks := p.popCommentTasks(); len(tasks) != 0 {
		t.Errorf("expected empty pop, got %+v", tasks)
	}
}
