package plugins

import (
	"os"
	"strings"
	"testing"
)

// TestInjectHooksOnRealBundle 对真实视频号 JS bundle 执行注入，
// 验证三处钩子都能命中且产物包含预期代码（语法由外部 node --check 复核）
func TestInjectHooksOnRealBundle(t *testing.T) {
	raw, err := os.ReadFile("testdata/wxjs_sample.js")
	if err != nil {
		t.Skip("testdata/wxjs_sample.js 不存在，跳过")
	}
	body := string(raw)

	if !qqMediaRegex.MatchString(body) {
		t.Fatal("get media() 正则在真实 bundle 中未命中")
	}
	if !qqCommentRegex.MatchString(body) {
		t.Fatal("finderGetCommentDetail 正则在真实 bundle 中未命中")
	}
	if !qqCommentListRegex.MatchString(body) {
		t.Fatal("finderGetCommentList 正则在真实 bundle 中未命中")
	}
	if got := len(qqPostRegex.FindAllStringIndex(body, -1)); got != 2 {
		t.Fatalf("post 总出口在真实 bundle 中应命中 2 处，实际 %d", got)
	}

	out := injectHooks(body)

	for _, marker := range []string{
		"__res_comment_poller",
		"__resGetCommentList",
		"__resCommentListTpl",
		"__resGetComment",
		"__resCommentDetailTpl",
		"res-downloader/wechat?type=2",
		"__resActiveCommentRequests",
		"identitySource:\"feed_model\"",
		"if(this.sessionBuffer) __sb=String(this.sessionBuffer)",
		"__resGetCommentListSource = \"native_method\"",
		"typeof this.finderGetCommentList === \"function\"",
		"expectedUrlSign",
		"finderBasereq = Object.assign",
		"res-downloader/wechat?type=9",
	} {
		if !strings.Contains(out, marker) {
			t.Errorf("注入产物缺少标记: %s", marker)
		}
	}

	// 输出供 node --check 语法复核
	if err := os.WriteFile("/tmp/wxjs_injected.js", []byte(out), 0644); err != nil {
		t.Fatal(err)
	}
	t.Log("injected bundle written to /tmp/wxjs_injected.js")
}

// 微信 4.1.11（client_version=4066647307）把 post 的 catch 错误枚举由
// Ce.JSAPI_UNKNOWN 改成 JstransferErr.JSAPI_UNKNOWN。总出口注入只应依赖
// post -> invoke 的稳定前缀，并完整保留微信自己的 catch 实现。
func TestInjectHooksOnWechat411PostShape(t *testing.T) {
	body := `
		class ApiA {
			async post(n){try{return await this.invoke(n)}catch(t){return this.createResp(JstransferErr.JSAPI_UNKNOWN,"JSAPI_UNKNOWN",{})}}
		}
		class ApiB {
			async post(n){try{return await this.invoke(n)}catch(t){return this.createResp(JstransferErr.JSAPI_UNKNOWN,"JSAPI_UNKNOWN",{})}}
		}
	`
	if got := len(qqPostRegex.FindAllStringIndex(body, -1)); got != 2 {
		t.Fatalf("微信 4.1.11 post 总出口应命中 2 处，实际 %d", got)
	}

	out := injectHooks(body)
	if got := strings.Count(out, "res-downloader/wechat?type=9"); got != 2 {
		t.Fatalf("微信 4.1.11 post 钩子应注入 2 处，实际 %d", got)
	}
	if got := strings.Count(out, `JstransferErr.JSAPI_UNKNOWN`); got != 2 {
		t.Fatalf("原始 catch 错误枚举应完整保留 2 处，实际 %d", got)
	}
	if strings.Contains(out, `Ce.JSAPI_UNKNOWN`) {
		t.Fatal("新版注入产物不应重新写入旧版错误枚举")
	}
	if err := os.WriteFile("/tmp/wxjs_4_1_11_injected.js", []byte(out), 0644); err != nil {
		t.Fatal(err)
	}
}

// TestInjectHooksOnExternalBundle 供升级取证时对刚从客户端缓存或代理保存的
// 完整 bundle 做复核：
// WXJS_BUNDLE_PATH=/tmp/latest.js go test ./core/plugins -run ExternalBundle -v
func TestInjectHooksOnExternalBundle(t *testing.T) {
	path := os.Getenv("WXJS_BUNDLE_PATH")
	if path == "" {
		t.Skip("未设置 WXJS_BUNDLE_PATH")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)

	checks := []struct {
		name string
		re   interface{ MatchString(string) bool }
	}{
		{"get media()", qqMediaRegex},
		{"finderGetCommentDetail", qqCommentRegex},
		{"finderGetCommentList", qqCommentListRegex},
	}
	for _, check := range checks {
		if !check.re.MatchString(body) {
			t.Fatalf("最新 bundle 未命中 %s", check.name)
		}
	}
	if got := len(qqPostRegex.FindAllStringIndex(body, -1)); got != 2 {
		t.Fatalf("最新 bundle 的 post 总出口应命中 2 处，实际 %d", got)
	}

	out := injectHooks(body)
	if got := strings.Count(out, "res-downloader/wechat?type=9"); got != 2 {
		t.Fatalf("最新 bundle 应注入 2 处 post 回传，实际 %d", got)
	}
	if err := os.WriteFile("/tmp/wxjs_latest_injected.js", []byte(out), 0644); err != nil {
		t.Fatal(err)
	}
	t.Logf("latest bundle validated: bytes=%d", len(raw))
}
