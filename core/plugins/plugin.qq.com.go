package plugins

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/elazarl/goproxy"
	gonanoid "github.com/matoous/go-nanoid/v2"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"res-downloader/core/shared"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var qqMediaRegex = regexp.MustCompile(`get\s*media\(\)\{`)
var qqCommentRegex = regexp.MustCompile(`async\s*finderGetCommentDetail\((\w+)\)\s*\{return(.*?)\s*}\s*async`)
var qqCommentListRegex = regexp.MustCompile(`async\s*finderGetCommentList\((\w+)\)\s*\{return(.*?)\s*}\s*async`)
var qqFinderVideoHostRegex = regexp.MustCompile(`^finder[a-z0-9-]*\.video\.qq\.com$`)

// qqPostRegex 只匹配 API 客户端 post -> invoke 的稳定前缀（两个客户端类各一份）。
// 不再匹配 catch/createResp：微信 4.1.11 将错误枚举从 Ce.JSAPI_UNKNOWN
// 改成 JstransferErr.JSAPI_UNKNOWN，硬编码该实现细节会使总出口完全失配。
// 第 1 组是 post 参数，第 2 组是传给 invoke 的参数；原始 catch 会原样保留。
var qqPostRegex = regexp.MustCompile(`async\s*post\(([\w$]+)\)\s*\{\s*try\s*\{\s*return\s+await\s+this\.invoke\(([\w$]+)\)\s*\}`)

// injectNonce 每次进程启动生成一次，用于注入 JS 的 URL 缓存击穿
var injectNonce = strconv.FormatInt(time.Now().Unix(), 36)

const commentTaskTimeout = 30 * time.Second

var (
	errCommentIdentityUnavailable = errors.New("comment_identity_unavailable")
	errCommentTaskActive          = errors.New("comment_task_active")
)

// commentTask 表示一个待拉取评论的任务（由前端"获取评论"按钮触发，
// 经 /api/get-comments 入队，等待页面内注入 JS 轮询取走）
type commentTask struct {
	RequestId     string `json:"requestId"`
	ObjectId      string `json:"objectId"`
	NonceId       string `json:"nonceId"`
	SessionBuffer string `json:"sessionBuffer"`
	ResId         string `json:"resId"`
	UrlSign       string `json:"urlSign"`
	CreatedAt     int64  `json:"createdAt"`
}

// commentIdentity 一条资源的权威身份（来自同一 feed 模型或推荐列表响应，
// 三者同源：objectId + objectNonceId + sessionBuffer。
// 服务端按 sessionBuffer 定位 feed，三者缺一不可）
type commentIdentity struct {
	ObjectId      string
	NonceId       string
	SessionBuffer string
	Source        string
	LearnedAt     int64
	Complete      bool
}

func (id commentIdentity) isComplete() bool {
	return id.Complete && id.ObjectId != "" && id.NonceId != "" && id.SessionBuffer != ""
}

type commentTaskStatus struct {
	RequestId string `json:"requestId"`
	ResId     string `json:"resId"`
	State     string `json:"state"`
	Code      string `json:"code,omitempty"`
	Count     int    `json:"count,omitempty"`
	UpdatedAt int64  `json:"updatedAt"`
}

type QqPlugin struct {
	bridge *shared.Bridge

	taskMux           sync.Mutex
	commentTasks      map[string]commentTask     // requestId -> task（待页面轮询取走）
	activeTasks       map[string]commentTask     // resId -> task（排队或执行中）
	delivered         map[string]string          // objectId / n:nonce -> resId（兼容强身份响应关联）
	signMap           map[string]commentIdentity // urlSign -> 身份；只有 Complete=true 可用于主动请求
	authoritativeSeen map[string]bool            // post recommend 同源资源独立去重
	debugURLSeen      map[string]bool            // 抑制脱敏 URL 结构诊断的重复日志
}

func (p *QqPlugin) SetBridge(bridge *shared.Bridge) {
	p.bridge = bridge
}

// logf 诊断日志（写入应用日志文件，便于排查嗅探链路）
func (p *QqPlugin) logf(format string, v ...interface{}) {
	if p.bridge != nil && p.bridge.Log != nil {
		p.bridge.Log("[QqPlugin] "+format, v...)
	}
}

func shortRef(value string) string {
	if value == "" {
		return "none"
	}
	sum := sha256.Sum256([]byte(value))
	return fmt.Sprintf("len:%d sha:%x", len(value), sum[:4])
}

// safeURLShape 只记录定位 URL 的结构与短哈希，不记录完整路径、查询值或令牌。
func safeURLShape(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "invalid"
	}
	query := u.Query()
	keys := make([]string, 0, len(query))
	for key := range query {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return fmt.Sprintf("host:%s path:%s keys:%s enc:%s token:%s file:%s",
		u.Hostname(), shortRef(u.EscapedPath()), strings.Join(keys, ","),
		shortRef(query.Get("encfilekey")), shortRef(query.Get("token")), shortRef(query.Get("filekey")))
}

func (p *QqPlugin) logURLShapeOnce(source, rawURL, urlSign, objectID string) {
	key := source + ":" + urlSign + ":" + objectID
	p.taskMux.Lock()
	if p.debugURLSeen == nil {
		p.debugURLSeen = make(map[string]bool)
	}
	if p.debugURLSeen[key] {
		p.taskMux.Unlock()
		return
	}
	p.debugURLSeen[key] = true
	p.taskMux.Unlock()
	p.logf("url_shape source=%s sign=%s object=%s %s", source, shortRef(urlSign), shortRef(objectID), safeURLShape(rawURL))
}

func (p *QqPlugin) claimAuthoritativeResource(urlSign string) bool {
	p.taskMux.Lock()
	defer p.taskMux.Unlock()
	if p.authoritativeSeen == nil {
		p.authoritativeSeen = make(map[string]bool)
	}
	if p.authoritativeSeen[urlSign] {
		return false
	}
	p.authoritativeSeen[urlSign] = true
	return true
}

func (p *QqPlugin) sendCommentStatus(task commentTask, state, code string, count int) {
	if p.bridge == nil || p.bridge.Send == nil {
		return
	}
	p.bridge.Send("commentTaskStatus", commentTaskStatus{
		RequestId: task.RequestId,
		ResId:     task.ResId,
		State:     state,
		Code:      code,
		Count:     count,
		UpdatedAt: time.Now().UnixMilli(),
	})
}

func (p *QqPlugin) cleanupExpiredTasksLocked(now time.Time) {
	for resId, task := range p.activeTasks {
		if now.Sub(time.UnixMilli(task.CreatedAt)) < commentTaskTimeout {
			continue
		}
		delete(p.activeTasks, resId)
		delete(p.commentTasks, task.RequestId)
		delete(p.delivered, task.ObjectId)
		delete(p.delivered, "n:"+task.NonceId)
	}
}

// AddCommentTask 只允许同一个 feed 原子学习到的完整三元组入队。
// 身份可来自 recommend post 响应，或 get media() 所在 feed 模型的精确字段；
// 传入的 urlSign 是资源与 feed 身份之间唯一的选择键，组件扫描值不参与选择。
func (p *QqPlugin) AddCommentTask(urlSign, resId string) (commentTask, error) {
	p.taskMux.Lock()
	if p.activeTasks == nil {
		p.activeTasks = make(map[string]commentTask)
	}
	if p.delivered == nil {
		p.delivered = make(map[string]string)
	}
	p.cleanupExpiredTasksLocked(time.Now())
	identity := p.signMap[urlSign]
	if !identity.isComplete() {
		p.taskMux.Unlock()
		p.logf("task_rejected code=identity_unavailable sign=%s hasObject=%v hasNonce=%v hasSB=%v complete=%v",
			shortRef(urlSign), identity.ObjectId != "", identity.NonceId != "", identity.SessionBuffer != "", identity.Complete)
		return commentTask{}, errCommentIdentityUnavailable
	}
	if existing, ok := p.activeTasks[resId]; ok {
		p.taskMux.Unlock()
		p.logf("task_rejected code=task_active request=%s res=%s", shortRef(existing.RequestId), shortRef(resId))
		return existing, errCommentTaskActive
	}
	if p.commentTasks == nil {
		p.commentTasks = make(map[string]commentTask)
	}
	requestId, err := gonanoid.New()
	if err != nil {
		requestId = fmt.Sprintf("comment-%d", time.Now().UnixNano())
	}
	task := commentTask{
		RequestId:     requestId,
		ObjectId:      identity.ObjectId,
		NonceId:       identity.NonceId,
		SessionBuffer: identity.SessionBuffer,
		ResId:         resId,
		UrlSign:       urlSign,
		CreatedAt:     time.Now().UnixMilli(),
	}
	p.commentTasks[requestId] = task
	p.activeTasks[resId] = task
	p.taskMux.Unlock()

	p.logf("task_queued request=%s res=%s sign=%s hasTriple=true object=%s nonce=%s sb=%s source=%s",
		shortRef(requestId), shortRef(resId), shortRef(urlSign), shortRef(task.ObjectId), shortRef(task.NonceId), shortRef(task.SessionBuffer), identity.Source)
	p.sendCommentStatus(task, "queued", "", 0)
	return task, nil
}

// popCommentTasks 取出全部待处理任务（页面 JS 轮询时调用），并记录 resId 供回传关联
func (p *QqPlugin) popCommentTasks() []commentTask {
	p.taskMux.Lock()
	if len(p.commentTasks) == 0 {
		p.taskMux.Unlock()
		return nil
	}
	if p.delivered == nil {
		p.delivered = make(map[string]string)
	}
	tasks := make([]commentTask, 0, len(p.commentTasks))
	for k, t := range p.commentTasks {
		tasks = append(tasks, t)
		if t.ObjectId != "" {
			p.delivered[t.ObjectId] = t.ResId
		}
		if t.NonceId != "" {
			p.delivered["n:"+t.NonceId] = t.ResId
		}
		delete(p.commentTasks, k)
	}
	p.taskMux.Unlock()
	for _, task := range tasks {
		p.logf("task_picked request=%s res=%s hasTriple=%v", shortRef(task.RequestId), shortRef(task.ResId), task.ObjectId != "" && task.NonceId != "" && task.SessionBuffer != "")
		p.sendCommentStatus(task, "running", "", 0)
	}
	return tasks
}

func (p *QqPlugin) activeTask(requestId, resId string) (commentTask, bool) {
	p.taskMux.Lock()
	defer p.taskMux.Unlock()
	task, ok := p.activeTasks[resId]
	if !ok || task.RequestId != requestId {
		return commentTask{}, false
	}
	return task, true
}

func (p *QqPlugin) completeTask(task commentTask) {
	p.taskMux.Lock()
	defer p.taskMux.Unlock()
	if current, ok := p.activeTasks[task.ResId]; ok && current.RequestId == task.RequestId {
		delete(p.activeTasks, task.ResId)
	}
	delete(p.commentTasks, task.RequestId)
	delete(p.delivered, task.ObjectId)
	delete(p.delivered, "n:"+task.NonceId)
}

// resIdOf 根据 objectId 查找关联的资源 Id（无则空串）
func (p *QqPlugin) resIdOf(objectId string) string {
	p.taskMux.Lock()
	defer p.taskMux.Unlock()
	if p.delivered == nil || objectId == "" {
		return ""
	}
	return p.delivered[objectId]
}

// resIdOfNonce 根据 objectNonceId 查找关联的资源 Id（无则空串）
func (p *QqPlugin) resIdOfNonce(nonceId string) string {
	p.taskMux.Lock()
	defer p.taskMux.Unlock()
	if p.delivered == nil || nonceId == "" {
		return ""
	}
	return p.delivered["n:"+nonceId]
}

// learnSignMapping 记录 urlSign -> 身份。
// 只有同一次上报同时带齐三元组时才标记 Complete；不完整来源不会污染已存在的完整三元组。
func (p *QqPlugin) learnSignMapping(urlSign string, id commentIdentity) bool {
	if urlSign == "" || (id.ObjectId == "" && id.NonceId == "" && id.SessionBuffer == "") {
		return false
	}
	p.taskMux.Lock()
	defer p.taskMux.Unlock()
	if p.signMap == nil {
		p.signMap = make(map[string]commentIdentity)
	}
	now := time.Now().UnixMilli()
	if id.ObjectId != "" && id.NonceId != "" && id.SessionBuffer != "" {
		old := p.signMap[urlSign]
		changed := !old.isComplete() || old.ObjectId != id.ObjectId || old.NonceId != id.NonceId ||
			old.SessionBuffer != id.SessionBuffer || old.Source != id.Source
		id.Complete = true
		id.LearnedAt = now
		p.signMap[urlSign] = id
		return changed
	}
	old := p.signMap[urlSign]
	if old.isComplete() {
		return false
	}
	previousObject, previousNonce, previousSession, previousSource := old.ObjectId, old.NonceId, old.SessionBuffer, old.Source
	if id.ObjectId != "" {
		old.ObjectId = id.ObjectId
	}
	if id.NonceId != "" {
		old.NonceId = id.NonceId
	}
	if id.SessionBuffer != "" {
		old.SessionBuffer = id.SessionBuffer
	}
	if id.Source != "" {
		old.Source = id.Source
	}
	old.LearnedAt = now
	old.Complete = false
	p.signMap[urlSign] = old
	return previousObject != old.ObjectId || previousNonce != old.NonceId ||
		previousSession != old.SessionBuffer || previousSource != old.Source
}

// ResolveIdentity 按资源 UrlSign 反查已学习到的权威身份
func (p *QqPlugin) ResolveIdentity(urlSign string) commentIdentity {
	p.taskMux.Lock()
	defer p.taskMux.Unlock()
	if p.signMap == nil {
		return commentIdentity{}
	}
	return p.signMap[urlSign]
}

func (p *QqPlugin) Domains() []string {
	return []string{"qq.com"}
}

func isFinderVideoHost(hostPort string) bool {
	host := strings.ToLower(strings.TrimSpace(hostPort))
	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	}
	host = strings.TrimSuffix(host, ".")
	return qqFinderVideoHostRegex.MatchString(host)
}

func (p *QqPlugin) OnRequest(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
	if strings.Contains(r.Host, "qq.com") && strings.Contains(r.URL.Path, "/res-downloader/wechat") {
		reqType := r.URL.Query().Get("type")
		switch reqType {
		case "3": // 页面注入 JS 轮询：取走待拉取评论的任务
			return r, p.buildTasksResponse(r)
		case "4": // 页面注入 JS 回传：评论数据
			p.handleCommentRequest(r)
			return r, p.buildEmptyResponse(r)
		case "5": // 页面注入 JS 回传：主动调用评论接口失败的原因
			p.handleCommentExecutionReport(r)
			return r, p.buildEmptyResponse(r)
		case "6": // 兼容旧脚本；该通道已证实存在竞态，不再学习身份
			p.handleIdentityReport(r)
			return r, p.buildEmptyResponse(r)
		case "7": // 执行器回传：detail 换得的 nonce 与任务 objectId 的配对
			p.handleNoncePair(r)
			return r, p.buildEmptyResponse(r)
		case "8": // 兼容旧脚本；推荐身份统一由 post 总出口 type=9 学习
			p.handleRecommendReport(r)
			return r, p.buildEmptyResponse(r)
		case "9": // post 总出口拦截：推荐/评论列表/评论详情的解码后数据
			p.handlePostReport(r)
			return r, p.buildEmptyResponse(r)
		}
		if p.bridge.GetConfig("WxAction").(bool) && r.URL.Query().Get("type") == "1" {
			return p.handleWechatRequest(r, ctx)
		} else if !p.bridge.GetConfig("WxAction").(bool) && r.URL.Query().Get("type") == "2" {
			return p.handleWechatRequest(r, ctx)
		} else {
			return r, p.buildEmptyResponse(r)
		}
	}

	return nil, nil
}

type commentExecutionReport struct {
	RequestId string `json:"requestId"`
	ResId     string `json:"resId"`
	Code      string `json:"code"`
	Stage     string `json:"stage"`
	Error     string `json:"error"`
}

func normalizeExecutionCode(code, detail string) string {
	switch code {
	case "identity_fields_missing", "list_not_captured", "request_failed", "login_expired":
		return code
	}
	lower := strings.ToLower(detail)
	if (strings.Contains(lower, "login") || strings.Contains(lower, "auth") || strings.Contains(lower, "session")) &&
		(strings.Contains(lower, "expire") || strings.Contains(lower, "invalid")) {
		return "login_expired"
	}
	return "request_failed"
}

func normalizeExecutionStage(stage string) string {
	switch stage {
	case "preflight", "invoke", "response":
		return stage
	default:
		return "unknown"
	}
}

func (p *QqPlugin) handleCommentExecutionReport(r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		return
	}
	var report commentExecutionReport
	if err := json.Unmarshal(body, &report); err != nil {
		p.logf("task_failed code=invalid_execution_report bytes=%d", len(body))
		return
	}
	code := normalizeExecutionCode(report.Code, report.Error)
	stage := normalizeExecutionStage(report.Stage)
	task, ok := p.activeTask(report.RequestId, report.ResId)
	if !ok {
		p.logf("task_failed code=unmatched_execution_report request=%s res=%s stage=%s", shortRef(report.RequestId), shortRef(report.ResId), stage)
		return
	}
	p.completeTask(task)
	p.logf("task_failed request=%s res=%s code=%s stage=%s detail=%s", shortRef(task.RequestId), shortRef(task.ResId), code, stage, shortRef(report.Error))
	p.sendCommentStatus(task, "failed", code, 0)
}

// buildTasksResponse 返回当前待拉取评论的任务列表（JSON），
// 带 CORS 头以便页面内跨域 fetch 可读
func (p *QqPlugin) buildTasksResponse(r *http.Request) *http.Response {
	tasks := p.popCommentTasks()
	if tasks == nil {
		tasks = []commentTask{}
	}
	body, err := json.Marshal(tasks)
	if err != nil {
		return p.buildEmptyResponse(r)
	}
	resp := &http.Response{
		Status:        http.StatusText(http.StatusOK),
		StatusCode:    http.StatusOK,
		Header:        make(http.Header),
		Body:          io.NopCloser(bytes.NewReader(body)),
		ContentLength: int64(len(body)),
		Request:       r,
	}
	resp.Header.Set("Content-Type", "application/json; charset=utf-8")
	resp.Header.Set("Access-Control-Allow-Origin", "*")
	return resp
}

// handleCommentRequest 解析页面回传的评论数据并推送给前端
func (p *QqPlugin) handleCommentRequest(r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		return
	}
	go p.handleComments(body)
}

// handlePostReport 处理 type=9：post 总出口拦截的数据，按接口名路由
func (p *QqPlugin) handlePostReport(r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		return
	}
	var payload struct {
		Name            string          `json:"name"`
		Arg             string          `json:"arg"`
		Data            json.RawMessage `json:"data"`
		RequestId       string          `json:"requestId"`
		ResId           string          `json:"resId"`
		ExpectedUrlSign string          `json:"expectedUrlSign"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		p.logf("post_report_rejected code=invalid_json bytes=%d", len(body))
		return
	}

	switch payload.Name {
	case "FinderGetRecommend":
		if len(payload.Data) == 0 {
			p.logf("identity_batch source=post_recommend code=missing_data")
			return
		}
		// 批量权威身份（复用推荐提取逻辑）
		var d map[string]interface{}
		if err := json.Unmarshal(payload.Data, &d); err != nil {
			p.logf("identity_batch source=post_recommend code=invalid_data")
			return
		}
		feeds := extractFeedObjects(d)
		complete, missingSB := 0, 0
		for _, f := range feeds {
			identity := commentIdentity{ObjectId: f.objectId, NonceId: f.nonceId, SessionBuffer: f.sessionBuffer, Source: "post_recommend"}
			p.learnSignMapping(f.urlSign, identity)
			p.logURLShapeOnce("recommend", f.rawURL, f.urlSign, f.objectId)
			// 直接从同一个 post recommend feed 生成资源，确保资源 URL 与
			// objectId/nonceId/sessionBuffer 天然同源；不再跨 CDN URL 猜测映射。
			if f.objectDesc != nil {
				wrapped, err := json.Marshal(map[string]interface{}{
					"od": f.objectDesc, "oid": f.objectId, "onid": f.nonceId,
				})
				if err == nil {
					p.handleMedia(wrapped, "9")
				}
			}
			if identity.ObjectId != "" && identity.NonceId != "" && identity.SessionBuffer != "" {
				complete++
			}
			if identity.SessionBuffer == "" {
				missingSB++
			}
		}
		p.logf("identity_batch source=post_recommend feeds=%d complete=%d missingSB=%d", len(feeds), complete, missingSB)
	case "FinderGetCommentDetail", "FinderGetCommentList":
		if len(payload.Data) == 0 {
			if payload.RequestId != "" {
				if task, ok := p.activeTask(payload.RequestId, payload.ResId); ok {
					p.completeTask(task)
					p.logf("task_failed request=%s res=%s code=response_data_missing", shortRef(task.RequestId), shortRef(task.ResId))
					p.sendCommentStatus(task, "failed", "request_failed", 0)
				}
			}
			return
		}
		// 合成 handleComments 载荷复用解析与事件推送
		var data map[string]interface{}
		if err := json.Unmarshal(payload.Data, &data); err != nil {
			p.logf("comment_response_rejected code=invalid_data request=%s", shortRef(payload.RequestId))
			return
		}
		oid := ""
		if obj, ok := data["object"].(map[string]interface{}); ok {
			switch id := obj["id"].(type) {
			case string:
				oid = id
			case float64:
				oid = strconv.FormatFloat(id, 'f', -1, 64)
			}
		}
		out, _ := json.Marshal(map[string]interface{}{
			"name":            payload.Name,
			"requestId":       payload.RequestId,
			"resId":           payload.ResId,
			"expectedUrlSign": payload.ExpectedUrlSign,
			"objectId":        oid,
			"arg":             payload.Arg,
			"data":            data,
		})
		go p.handleComments(out)
	}
}

// handleRecommendReport 处理 type=8：从 FinderGetRecommend 响应中批量提取
// 视频对象（id + objectNonceId + objectDesc 中优先的视频媒体 URL），建立
// urlSign -> {objectId, nonceId} 权威映射（服务端数据，严格同源、随会话新鲜）
func (p *QqPlugin) handleRecommendReport(r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		return
	}
	var payload struct {
		Data map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return
	}

	feeds := extractFeedObjects(payload.Data)
	p.logf("legacy_recommend_ignored feeds=%d reason=post_exit_is_authoritative", len(feeds))
}

type feedIdentity struct {
	urlSign       string
	rawURL        string
	objectDesc    map[string]interface{}
	objectId      string
	nonceId       string
	sessionBuffer string
}

// preferredObjectDescMedia 选择用于展示与评论身份关联的主媒体。
// 微信 4.1.11 的 recommend objectDesc.media 可能把图片放在第 0 项，
// 而实际视频位于后续项；优先选择 Finder CDN 的非图片媒体，并返回
// 把该项移到首位的浅拷贝，供既有 handleMedia 解析逻辑复用。
func preferredObjectDescMedia(desc map[string]interface{}) (map[string]interface{}, string) {
	mediaArr, ok := desc["media"].([]interface{})
	if !ok || len(mediaArr) == 0 {
		return desc, ""
	}

	preferred, nonImage, finder, fallback := -1, -1, -1, -1
	for i, value := range mediaArr {
		media, ok := value.(map[string]interface{})
		if !ok {
			continue
		}
		rawURL, _ := media["url"].(string)
		if rawURL == "" {
			continue
		}
		if fallback == -1 {
			fallback = i
		}
		mediaType, _ := media["mediaType"].(float64)
		isImage := mediaType == 9
		parsed, err := url.Parse(rawURL)
		isFinder := err == nil && isFinderVideoHost(parsed.Host)
		if isFinder && finder == -1 {
			finder = i
		}
		if !isImage && nonImage == -1 {
			nonImage = i
		}
		if isFinder && !isImage {
			preferred = i
			break
		}
	}
	if preferred == -1 {
		if nonImage != -1 {
			preferred = nonImage
		} else if finder != -1 {
			preferred = finder
		} else {
			preferred = fallback
		}
	}
	if preferred == -1 {
		return desc, ""
	}
	selected := mediaArr[preferred].(map[string]interface{})
	rawURL, _ := selected["url"].(string)
	if preferred == 0 {
		return desc, rawURL
	}

	ordered := make([]interface{}, 0, len(mediaArr))
	ordered = append(ordered, mediaArr[preferred])
	ordered = append(ordered, mediaArr[:preferred]...)
	ordered = append(ordered, mediaArr[preferred+1:]...)
	copyDesc := make(map[string]interface{}, len(desc))
	for key, value := range desc {
		copyDesc[key] = value
	}
	copyDesc["media"] = ordered
	return copyDesc, rawURL
}

// extractFeedObjects 从推荐响应中多路径探测视频对象数组
func extractFeedObjects(data map[string]interface{}) []feedIdentity {
	var arr []interface{}
	for _, key := range []string{"object", "objects", "feedList", "feeds", "list", "items"} {
		if v, ok := data[key].([]interface{}); ok {
			arr = v
			break
		}
	}
	if arr == nil {
		return nil
	}

	var out []feedIdentity
	for _, item := range arr {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		var fi feedIdentity
		switch id := m["id"].(type) {
		case string:
			fi.objectId = id
		case float64:
			fi.objectId = strconv.FormatFloat(id, 'f', -1, 64)
		}
		if v, ok := m["objectNonceId"].(string); ok {
			fi.nonceId = v
		}
		if v, ok := m["sessionBuffer"].(string); ok {
			fi.sessionBuffer = v
		}
		if desc, ok := m["objectDesc"].(map[string]interface{}); ok {
			fi.objectDesc, fi.rawURL = preferredObjectDescMedia(desc)
			if fi.rawURL != "" {
				fi.urlSign = shared.Md5(fi.rawURL)
			}
		}
		if fi.urlSign != "" {
			out = append(out, fi)
		}
	}
	return out
}

// handleNoncePair 处理 type=7：执行器用 detail 换得新鲜 nonce 后，
// 回传 {objectId, nonceId}，据此建立 nonce -> resId 的回传关联，
// 使随后的 list 响应（只有 nonce，没有 objectid）能挂到正确资源
func (p *QqPlugin) handleNoncePair(r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		return
	}
	var payload struct {
		ObjectId string `json:"objectId"`
		NonceId  string `json:"nonceId"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || payload.ObjectId == "" || payload.NonceId == "" {
		return
	}
	resId := p.resIdOf(payload.ObjectId)
	if resId == "" {
		return
	}
	p.taskMux.Lock()
	defer p.taskMux.Unlock()
	if p.delivered != nil {
		p.delivered["n:"+payload.NonceId] = resId
	}
	p.logf("legacy_nonce_pair object=%s nonce=%s res=%s", shortRef(payload.ObjectId), shortRef(payload.NonceId), shortRef(resId))
}

// handleIdentityReport 兼容旧注入脚本。type=6 已被真实实验确认存在竞态，
// 因此只消费请求并记录忽略原因，绝不写入可用于主动请求的身份映射。
func (p *QqPlugin) handleIdentityReport(r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		return
	}
	p.logf("legacy_identity_ignored channel=type6 bytes=%d reason=known_race", len(body))
}

func (p *QqPlugin) OnResponse(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
	if resp.StatusCode != 200 && resp.StatusCode != 206 {
		return nil
	}

	host := resp.Request.Host
	Path := resp.Request.URL.Path

	classify, _ := p.bridge.TypeSuffix(resp.Header.Get("Content-Type"))
	if classify == "video" && isFinderVideoHost(host) {
		if strings.Contains(resp.Request.Header.Get("Origin"), "mp.weixin.qq.com") {
			return nil
		}
		return resp
	}

	if strings.HasSuffix(host, "channels.weixin.qq.com") &&
		(strings.Contains(Path, "/web/pages/feed") || strings.Contains(Path, "/web/pages/home")) {
		p.logf("page version swap: %s%s", host, Path)
		return p.replaceWxJsContent(resp, ".js\"", ".js?v="+p.v()+"\"")
	}

	if strings.HasSuffix(host, "res.wx.qq.com") {
		respTemp := resp
		is := false
		if strings.HasSuffix(respTemp.Request.URL.RequestURI(), ".js?v="+p.v()) {
			respTemp = p.replaceWxJsContent(respTemp, ".js\"", ".js?v="+p.v()+"\"")
			is = true
		}

		if strings.Contains(Path, "web/web-finder/res/js/virtual_svg-icons-register.publish") {
			body, err := io.ReadAll(respTemp.Body)
			if err != nil {
				return respTemp
			}
			bodyStr := string(body)
			mediaMatches := len(qqMediaRegex.FindAllStringIndex(bodyStr, -1))
			detailMatches := len(qqCommentRegex.FindAllStringIndex(bodyStr, -1))
			listMatches := len(qqCommentListRegex.FindAllStringIndex(bodyStr, -1))
			postMatches := len(qqPostRegex.FindAllStringIndex(bodyStr, -1))
			newBody := injectHooks(bodyStr)
			p.logf("hook_injected version=post-prefix-v2 bundle=%s media_matches=%d detail_matches=%d list_matches=%d post_matches=%d changed=%v",
				shortRef(Path), mediaMatches, detailMatches, listMatches, postMatches, newBody != bodyStr)
			if postMatches == 0 {
				p.logf("hook_incomplete code=post_shape_unmatched bundle=%s", shortRef(Path))
			}
			newBodyBytes := []byte(newBody)
			respTemp.Body = io.NopCloser(bytes.NewBuffer(newBodyBytes))
			respTemp.ContentLength = int64(len(newBodyBytes))
			respTemp.Header.Set("Content-Length", fmt.Sprintf("%d", len(newBodyBytes)))
			return respTemp
		}
		if is {
			return respTemp
		}
	}

	return nil
}

func (p *QqPlugin) handleWechatRequest(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return r, p.buildEmptyResponse(r)
	}

	go p.handleMedia(body, r.URL.Query().Get("type"))

	return r, p.buildEmptyResponse(r)
}

// handleMedia 解析页面回传的 objectDesc。
// typ 为回传通道：type=1 来自 feed 模型 get media() 钩子（只有明确标记且完整的
// id/objectNonceId/sessionBuffer 才可信）；
// type=2 来自 finderGetCommentDetail 钩子（oid/onid 与 objectDesc 同源于响应，可信）；
// type=9 来自 post(FinderGetRecommend)，objectDesc 与完整 feed 三元组同源，可信。
func (p *QqPlugin) handleMedia(body []byte, typ string) {
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		p.logf("handleMedia: invalid json, body=%d bytes", len(body))
		return
	}

	// 新版注入 JS 回传包装结构 {od: objectDesc, oid: objectId, onid: nonceId}；
	// 兼容旧版直接回传 objectDesc 的形式
	wrapperOid, wrapperOnid, wrapperSessionBuffer, wrapperIdentitySource := "", "", "", ""
	wrapped := false
	if od, ok := result["od"].(map[string]interface{}); ok {
		wrapped = true
		wrapperOid, _ = result["oid"].(string)
		wrapperOnid, _ = result["onid"].(string)
		wrapperSessionBuffer, _ = result["sb"].(string)
		wrapperIdentitySource, _ = result["identitySource"].(string)
		result = od
	}
	p.logf("media_received wrapped=%v object=%s nonce=%s bytes=%d", wrapped, shortRef(wrapperOid), shortRef(wrapperOnid), len(body))

	mediaArr, ok := result["media"].([]interface{})
	if !ok || len(mediaArr) == 0 {
		p.logf("media_rejected code=media_array_missing")
		return
	}

	firstMedia, ok := mediaArr[0].(map[string]interface{})
	if !ok {
		return
	}

	rawUrl, ok := firstMedia["url"].(string)
	if !ok || rawUrl == "" {
		return
	}

	urlSign := shared.Md5(rawUrl)
	// 诊断需覆盖已被资源列表去重的媒体请求，否则无法与新一轮 recommend
	// 中同一 feed 的 URL 结构做脱敏对照。logURLShapeOnce 自身会抑制重复。
	p.logURLShapeOnce("media", rawUrl, urlSign, wrapperOid)
	// type=1 只接受 get media() 所在 feed 模型的三个精确字段。真实 bundle
	// 证明该 getter 与 id/objectNonceId/sessionBuffer 位于同一个 bt 实例；
	// 不再扫描 Vue 组件，也不拼接跨对象字段。三元组缺一时绝不学习。
	modelIdentityComplete := typ == "1" && wrapperIdentitySource == "feed_model" &&
		wrapperOid != "" && wrapperOnid != "" && wrapperSessionBuffer != ""
	if modelIdentityComplete {
		identityChanged := p.learnSignMapping(urlSign, commentIdentity{
			ObjectId: wrapperOid, NonceId: wrapperOnid, SessionBuffer: wrapperSessionBuffer, Source: "feed_model",
		})
		if identityChanged {
			p.logf("media_identity_linked source=feed_model sign=%s object=%s nonce=%s sb=%s",
				shortRef(urlSign), shortRef(wrapperOid), shortRef(wrapperOnid), shortRef(wrapperSessionBuffer))
		}
	} else if typ == "1" && wrapperIdentitySource == "feed_model" {
		p.logf("media_identity_unavailable source=feed_model sign=%s hasObject=%v hasNonce=%v hasSB=%v",
			shortRef(urlSign), wrapperOid != "", wrapperOnid != "", wrapperSessionBuffer != "")
	}
	// type=2（detail 钩子）仍可记录不完整关联，但没有 sessionBuffer 时不会通过入队门槛。
	if typ == "2" && (wrapperOid != "" || wrapperOnid != "") {
		p.learnSignMapping(urlSign, commentIdentity{ObjectId: wrapperOid, NonceId: wrapperOnid, Source: "legacy_detail"})
	}
	// post recommend 同源资源必须至少向前端发送一次。它可能已被通用媒体层
	// 抢先标记，但通用事件不保证携带评论身份来源；因此使用插件内独立去重。
	if typ == "9" {
		if !p.claimAuthoritativeResource(urlSign) {
			p.logf("media_skipped code=authoritative_duplicate sign=%s", shortRef(urlSign))
			return
		}
	} else if p.bridge.MediaIsMarked(urlSign) {
		p.logf("media_skipped code=duplicate sign=%s", shortRef(urlSign))
		return
	}

	id, err := gonanoid.New()
	if err != nil {
		id = urlSign
	}

	res := shared.MediaInfo{
		Id:          id,
		Url:         rawUrl,
		UrlSign:     urlSign,
		CoverUrl:    "",
		Size:        0,
		Domain:      shared.GetTopLevelDomain(rawUrl),
		Classify:    "video",
		Suffix:      ".mp4",
		Status:      shared.DownloadStatusReady,
		SavePath:    "",
		DecodeKey:   "",
		OtherData:   map[string]string{},
		Description: "",
		ContentType: "video/mp4",
	}
	// wx_channel 仅用于前端识别来源；实际评论身份始终留在后端，
	// 并且只有同一 feed 原子取得的完整三元组才能通过入队校验。
	res.OtherData["wx_channel"] = "1"
	if typ == "9" {
		res.OtherData["wx_identity_source"] = "post_recommend"
		// recommend 可能只下发 feed 封面（mediaType=9），但它仍是
		// 与完整评论三元组同源的目标载体，不应被用户的媒体抓取类型过滤。
		// 保持实际图片分类，不把封面伪装成可下载的视频。
		res.OtherData["wx_comment_target"] = "1"
	} else if modelIdentityComplete {
		res.OtherData["wx_identity_source"] = "feed_model"
		res.OtherData["wx_comment_target"] = "1"
	}

	if mediaType, ok := firstMedia["mediaType"].(float64); ok && mediaType == 9 {
		res.Classify = "image"
		res.Suffix = ".png"
		res.ContentType = "image/png"
	}

	if typ != "9" {
		isAll, _ := p.bridge.GetResType("all")
		isImage, _ := p.bridge.GetResType("image")
		if res.Classify == "image" && !isImage && !isAll {
			return
		}

		isVideo, _ := p.bridge.GetResType("video")
		if res.Classify == "video" && !isVideo && !isAll {
			return
		}
	}

	if urlToken, ok := firstMedia["urlToken"].(string); ok {
		res.Url += urlToken
	}

	switch size := firstMedia["fileSize"].(type) {
	case float64:
		res.Size = size
	case string:
		if value, err := strconv.ParseFloat(size, 64); err == nil {
			res.Size = value
		}
	}

	if coverUrl, ok := firstMedia["coverUrl"].(string); ok {
		res.CoverUrl = coverUrl
	}

	if decodeKey, ok := firstMedia["decodeKey"].(string); ok {
		res.DecodeKey = decodeKey
	}

	if desc, ok := result["description"].(string); ok {
		res.Description = desc
	}

	// 记录视频号动态 id / nonceId，供"获取评论"功能定位目标。
	// 优先使用注入 JS 从页面组件/评论响应中提取的包装字段；
	// objectDesc 自身通常不含 id，候选键仅作兜底。
	// 同时记录顶层字段名（wx_debug_keys）便于排查结构差异。
	if wrapperOid != "" {
		res.OtherData["wx_object_id"] = wrapperOid
	}
	if wrapperOnid != "" {
		res.OtherData["wx_nonce_id"] = wrapperOnid
	}

	keys := make([]string, 0, len(result))
	for k := range result {
		keys = append(keys, k)
	}
	res.OtherData["wx_debug_keys"] = strings.Join(keys, ",")

	if res.OtherData["wx_object_id"] == "" {
		idCandidates := []string{"id", "objectId", "feedId", "exportId", "object_id"}
		for _, key := range idCandidates {
			switch id := result[key].(type) {
			case string:
				if id != "" {
					res.OtherData["wx_object_id"] = id
				}
			case float64:
				res.OtherData["wx_object_id"] = strconv.FormatFloat(id, 'f', -1, 64)
			}
			if res.OtherData["wx_object_id"] != "" {
				break
			}
		}
	}
	if res.OtherData["wx_nonce_id"] == "" {
		for _, key := range []string{"nonceId", "objectNonceId", "nonce_id"} {
			if v, ok := result[key].(string); ok && v != "" {
				res.OtherData["wx_nonce_id"] = v
				break
			}
		}
	}

	// 注意：nonceId 的数字前缀并非动态 ID（实测不符），不做派生。
	// 真实 objectId 的权威来源是评论响应 res.data.object.id（signMap 学习）。

	if spec, ok := firstMedia["spec"].([]interface{}); ok {
		var fileFormats []string
		for _, item := range spec {
			if m, ok := item.(map[string]interface{}); ok {
				if format, ok := m["fileFormat"].(string); ok {
					fileFormats = append(fileFormats, format)
				}
			}
		}
		res.OtherData["wx_file_formats"] = strings.Join(fileFormats, "#")
	}

	p.bridge.MarkMedia(urlSign)
	source := "media_hook"
	if typ == "9" {
		source = "post_recommend"
	}
	p.logf("media_emitted source=%s sign=%s object=%s classify=%s descriptionLength=%d", source, shortRef(urlSign), shortRef(res.OtherData["wx_object_id"]), res.Classify, len(res.Description))

	// handleWechatRequest 已在 goroutine 中执行 handleMedia；此处同步发送可避免
	// 冗余 goroutine 和事件顺序/测试读取的数据竞争。
	p.bridge.Send("newResources", res)
}

func (p *QqPlugin) buildEmptyResponse(r *http.Request) *http.Response {
	body := "The content does not exist"
	resp := &http.Response{
		Status:        http.StatusText(http.StatusOK),
		StatusCode:    http.StatusOK,
		Header:        make(http.Header),
		Body:          io.NopCloser(strings.NewReader(body)),
		ContentLength: int64(len(body)),
		Request:       r,
	}
	resp.Header.Set("Content-Type", "text/plain; charset=utf-8")
	return resp
}

func (p *QqPlugin) replaceWxJsContent(resp *http.Response, old, new string) *http.Response {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp
	}
	bodyString := string(body)
	newBodyString := strings.ReplaceAll(bodyString, old, new)
	newBodyBytes := []byte(newBodyString)
	resp.Body = io.NopCloser(bytes.NewBuffer(newBodyBytes))
	resp.ContentLength = int64(len(newBodyBytes))
	resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(newBodyBytes)))
	return resp
}

func (p *QqPlugin) v() string {
	// 版本号叠加启动时间戳，确保每次启动注入 JS 的 URL 都不同，
	// 避免浏览器/客户端 webview 使用缓存的旧注入代码
	return p.bridge.GetVersion() + "-" + injectNonce
}

// injectHooks 对视频号页面 JS bundle 执行三处钩子注入：
// 1. get media()：回传 objectDesc + 安装评论任务轮询器
// 2. finderGetCommentDetail：捕获函数引用与参数模板，回传动态信息/评论
// 3. finderGetCommentList：评论列表面板接口，同上
func injectHooks(bodyStr string) string {
	newBody := qqMediaRegex.
		ReplaceAllString(bodyStr, `
					get media(){
						if(this.objectDesc){
							window.__resCurrentMedia = this.objectDesc;
							var __oid="",__onid="",__sb="";
							try{
								// 当前唯一 get media() 位于视频号 feed 模型；这三个字段与
								// objectDesc/实际媒体天然属于同一个实例，不做组件字段扫描。
								if(this.id) __oid=String(this.id);
								if(this.objectNonceId) __onid=String(this.objectNonceId);
								if(this.sessionBuffer) __sb=String(this.sessionBuffer);
							}catch(e){}
							fetch("https://wxapp.tc.qq.com/res-downloader/wechat?type=1", {
							  method: "POST",
							  mode: "no-cors",
							  body: JSON.stringify({od:this.objectDesc,oid:__oid,onid:__onid,sb:__sb,identitySource:"feed_model"}),
							});
						};
						if(!window.__res_comment_poller){
							window.__res_comment_poller = 1;
							setInterval(function(){
								fetch("https://wxapp.tc.qq.com/res-downloader/wechat?type=3")
									.then(function(r){return r.json();})
									.then(function(list){
										(list||[]).forEach(function(t){
										var __report = function(code, stage, err){
											fetch("https://wxapp.tc.qq.com/res-downloader/wechat?type=5", {
											  method: "POST", mode: "no-cors",
											  body: JSON.stringify({requestId:t.requestId,resId:t.resId,code:code,stage:stage,error:String(err||"")}),
											});
										};
										var __run = async function(){
											try{
												var nonce = t.nonceId || "";
												if(!t.objectId || !nonce || !t.sessionBuffer){ __report("identity_fields_missing","preflight","required feed identity field missing"); return; }
												if(!window.__resGetCommentList){ __report("list_not_captured","preflight","expand comments once to capture list function"); return; }
												// 三件套定位：objectid + objectNonceId + sessionBuffer
												// 服务端按 sessionBuffer 定位 feed，模板里的必须换掉
												var p = Object.assign({}, window.__resCommentListTpl || {}, {objectNonceId: nonce, direction: 2});
												delete p.lastBuffer;
												p.objectid = t.objectId;
												if(Object.prototype.hasOwnProperty.call(p,"objectId")){ p.objectId = t.objectId; }
												p.sessionBuffer = t.sessionBuffer;
												p.finderBasereq = Object.assign({}, p.finderBasereq || {}, {objectBaseInfos: [{sessionBuffer: t.sessionBuffer}]});
												window.__resActiveCommentRequests = window.__resActiveCommentRequests || {};
											window.__resActiveCommentRequests[nonce+"::"+t.sessionBuffer] = {requestId:t.requestId,resId:t.resId,expectedUrlSign:t.urlSign};
												var __pp = await window.__resGetCommentList(p);
												if(!__pp || !__pp.data){ __report("request_failed","response","comment list returned no data"); }
											}catch(e){ __report("request_failed","invoke",e); }
											};
											__run();
										});
									}).catch(function(){});
							}, 3000);
						}

	`)

	newBody = qqCommentRegex.
		ReplaceAllString(newBody, `
					async finderGetCommentDetail($1) {
						var res = await$2;
						if(!window.__resGetComment){
							var __resSelf = this;
							window.__resGetComment = function(oid){ return __resSelf.finderGetCommentDetail(oid); };
						}
						try { window.__resCommentDetailTpl = $1; } catch(e) {}
						// 保留 WxAction=false 时既有的 detail 媒体嗅探通道；
						// 评论任务不使用 detail 的不完整身份；这里只保留媒体兼容通道。
						if (res?.data?.object?.objectDesc) {
							fetch("https://wxapp.tc.qq.com/res-downloader/wechat?type=2", {
							  method:"POST",mode:"no-cors",
							  body:JSON.stringify({
								od:res.data.object.objectDesc,
								oid:String(res.data.object.id||""),
								onid:String(res.data.object.objectNonceId||"")
							  })
							});
						}
						return res;
					}async
	`)

	newBody = qqCommentListRegex.
		ReplaceAllString(newBody, `
					async finderGetCommentList($1) {
						var res = await$2;
						if(!window.__resGetCommentList){
							var __resSelfL = this;
							window.__resGetCommentList = function(t){ return __resSelfL.finderGetCommentList(t); };
						}
						try { window.__resCommentListTpl = $1; } catch(e) {}
						return res;
					}async
	`)

	// 总出口钩子：修补两个 API 客户端的 post 方法，
	// 拦截全部 Finder 接口（推荐/评论列表/评论详情）的解码后数据，
	// 并在此层捕获函数引用与最终形态参数模板
	newBody = qqPostRegex.
		ReplaceAllString(newBody, `
					async post($1){
						try{
							var __r = await this.invoke($2);
							try{
									var __nm = $1 && $1.name || "";
									if(__nm === "FinderGetRecommend" || __nm === "FinderGetCommentList" || __nm === "FinderGetCommentDetail"){
										// FinderGetRecommend 首次完成时就能取得 API 客户端实例。
										// 优先保存原生 finderGetCommentList 方法，让自动任务无需用户
										// 先手动展开评论区，且保留客户端自己的 finderBasereq 合并逻辑。
										if(!window.__resGetCommentList && typeof this.finderGetCommentList === "function"){
											var __nativeListClient = this;
											window.__resGetCommentList = function(t){ return __nativeListClient.finderGetCommentList(t); };
											window.__resGetCommentListSource = "native_method";
										}
										if(__nm === "FinderGetCommentList"){
											if(!window.__resGetCommentList){ var __sp = this; window.__resGetCommentList = function(t){ return __sp.post({name:"FinderGetCommentList", data: t}); }; window.__resGetCommentListSource = "post_fallback"; }
										try{ window.__resCommentListTpl = $1.data; }catch(e){}
									}
									if(__nm === "FinderGetCommentDetail"){
										if(!window.__resGetComment){ var __sd = this; window.__resGetComment = function(t){ return __sd.post({name:"FinderGetCommentDetail", data: t}); }; }
										try{ window.__resCommentDetailTpl = $1.data; }catch(e){}
									}
									var __arg = "";
									try{ __arg = JSON.stringify($1.data); }catch(e){}
									var __taskCtx = null;
									try{
										if(__nm === "FinderGetCommentList" && $1.data && window.__resActiveCommentRequests){
											var __taskKey=String($1.data.objectNonceId||"")+"::"+String($1.data.sessionBuffer||"");
											__taskCtx = window.__resActiveCommentRequests[__taskKey] || null;
											if(__taskCtx){ delete window.__resActiveCommentRequests[__taskKey]; }
										}
									}catch(e){}
									fetch("https://wxapp.tc.qq.com/res-downloader/wechat?type=9", {
									  method: "POST",
									  mode: "no-cors",
									  body: JSON.stringify({
										name:__nm,arg:__arg,data:__r&&__r.data,
										requestId:__taskCtx&&__taskCtx.requestId||"",
										resId:__taskCtx&&__taskCtx.resId||"",
										expectedUrlSign:__taskCtx&&__taskCtx.expectedUrlSign||""
									  }),
									});
								}
							}catch(e){}
							return __r;
						}
	`)
	return newBody
}

// CommentItem 评论条目（尽力提取，字段可能为空）
type CommentItem struct {
	NickName  string  `json:"nickName"`
	Content   string  `json:"content"`
	LikeCount float64 `json:"likeCount"`
	CreatedAt float64 `json:"createdAt"`
	ReplyCnt  float64 `json:"replyCount"`
	Region    string  `json:"region"`
}

type commentRequestIdentity struct {
	ObjectId            string
	NonceId             string
	SessionBuffer       string
	NestedSessionBuffer string
}

func jsonString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	default:
		return ""
	}
}

func parseCommentRequestIdentity(arg string) (commentRequestIdentity, error) {
	var raw map[string]interface{}
	if arg == "" || json.Unmarshal([]byte(arg), &raw) != nil {
		return commentRequestIdentity{}, errors.New("invalid comment request arg")
	}
	id := commentRequestIdentity{
		ObjectId:      jsonString(raw["objectid"]),
		NonceId:       jsonString(raw["objectNonceId"]),
		SessionBuffer: jsonString(raw["sessionBuffer"]),
	}
	if base, ok := raw["finderBasereq"].(map[string]interface{}); ok {
		if infos, ok := base["objectBaseInfos"].([]interface{}); ok && len(infos) > 0 {
			if first, ok := infos[0].(map[string]interface{}); ok {
				id.NestedSessionBuffer = jsonString(first["sessionBuffer"])
			}
		}
	}
	return id, nil
}

func validateCommentRequest(task commentTask, arg string) string {
	id, err := parseCommentRequestIdentity(arg)
	if err != nil || id.ObjectId == "" || id.NonceId == "" || id.SessionBuffer == "" || id.NestedSessionBuffer == "" {
		return "identity_fields_missing"
	}
	if id.ObjectId != task.ObjectId || id.NonceId != task.NonceId || id.SessionBuffer != task.SessionBuffer || id.NestedSessionBuffer != task.SessionBuffer {
		return "target_mismatch"
	}
	return ""
}

func responseIdentity(data map[string]interface{}) (objectId, nonceId, urlSign string) {
	obj, ok := data["object"].(map[string]interface{})
	if !ok {
		return "", "", ""
	}
	objectId = jsonString(obj["id"])
	nonceId = jsonString(obj["objectNonceId"])
	desc, ok := obj["objectDesc"].(map[string]interface{})
	if !ok {
		return objectId, nonceId, ""
	}
	_, rawURL := preferredObjectDescMedia(desc)
	if rawURL != "" {
		urlSign = shared.Md5(rawURL)
	}
	return objectId, nonceId, urlSign
}

func commentList(data map[string]interface{}) ([]interface{}, bool) {
	if data == nil {
		return nil, false
	}
	for _, key := range []string{"commentInfo", "commentList", "comments", "comment"} {
		if list, ok := data[key].([]interface{}); ok {
			return list, true
		}
	}
	if obj, ok := data["object"].(map[string]interface{}); ok {
		for _, key := range []string{"commentInfo", "commentList", "comments"} {
			if list, ok := obj[key].([]interface{}); ok {
				return list, true
			}
		}
	}
	return nil, false
}

// handleComments 只把主动任务或带响应媒体 URL 的强关联结果推送给前端。
// 原始响应和完整请求参数不会离开 Go 解析层。
func (p *QqPlugin) handleComments(body []byte) {
	var payload struct {
		Name            string                 `json:"name"`
		RequestId       string                 `json:"requestId"`
		ResId           string                 `json:"resId"`
		ExpectedUrlSign string                 `json:"expectedUrlSign"`
		ObjectId        string                 `json:"objectId"`
		Arg             string                 `json:"arg"`
		Data            map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		p.logf("comment_response_rejected code=invalid_payload bytes=%d", len(body))
		return
	}

	responseObjectId, responseNonce, responseUrlSign := responseIdentity(payload.Data)
	if responseObjectId != "" {
		payload.ObjectId = responseObjectId
	}
	p.learnSignMapping(responseUrlSign, commentIdentity{ObjectId: payload.ObjectId, NonceId: responseNonce, Source: "legacy_comment_response"})

	list, found := commentList(payload.Data)
	comments := extractComments(payload.Data)
	resId := payload.ResId
	urlSign := responseUrlSign
	var task commentTask
	active := false
	if payload.RequestId != "" {
		task, active = p.activeTask(payload.RequestId, payload.ResId)
		if !active {
			p.logf("comment_response_rejected code=unmatched_task request=%s res=%s", shortRef(payload.RequestId), shortRef(payload.ResId))
			return
		}
		resId = task.ResId
		urlSign = task.UrlSign
		validationCode := validateCommentRequest(task, payload.Arg)
		if validationCode == "" && payload.ExpectedUrlSign != task.UrlSign {
			validationCode = "target_mismatch"
		}
		if validationCode == "" && responseUrlSign != "" && responseUrlSign != task.UrlSign {
			validationCode = "target_mismatch"
		}
		if validationCode != "" {
			p.completeTask(task)
			p.logf("task_failed request=%s res=%s code=%s responseSign=%s expectedSign=%s", shortRef(task.RequestId), shortRef(task.ResId), validationCode, shortRef(responseUrlSign), shortRef(task.UrlSign))
			p.sendCommentStatus(task, "failed", validationCode, 0)
			return
		}
		p.logf("request_verified request=%s res=%s object=%s nonce=%s sb=%s responseHasSign=%v",
			shortRef(task.RequestId), shortRef(task.ResId), shortRef(task.ObjectId), shortRef(task.NonceId), shortRef(task.SessionBuffer), responseUrlSign != "")
	} else {
		if !found || responseUrlSign == "" {
			p.logf("comment_response_ignored reason=no_active_task hasList=%v hasResponseSign=%v", found, responseUrlSign != "")
			return
		}
		if resId == "" {
			if requestIdentity, err := parseCommentRequestIdentity(payload.Arg); err == nil {
				resId = p.resIdOfNonce(requestIdentity.NonceId)
				if resId == "" {
					resId = p.resIdOf(requestIdentity.ObjectId)
				}
			}
		}
	}

	if !found {
		if active {
			p.completeTask(task)
			p.logf("task_failed request=%s res=%s code=parse_failed responseKeys=%d", shortRef(task.RequestId), shortRef(task.ResId), len(payload.Data))
			p.sendCommentStatus(task, "failed", "parse_failed", 0)
		}
		return
	}

	status := "success"
	if len(list) == 0 {
		status = "no_comments"
	} else if len(comments) < len(list) {
		status = "partial_success"
	}
	if p.bridge != nil && p.bridge.Send != nil {
		p.bridge.Send("newComments", map[string]interface{}{
			"requestId":      payload.RequestId,
			"objectId":       payload.ObjectId,
			"resId":          resId,
			"urlSign":        urlSign,
			"comments":       comments,
			"status":         status,
			"totalCount":     len(list),
			"targetVerified": active || responseUrlSign != "",
			"fetchedAt":      time.Now().UnixMilli(),
		})
	}
	if active {
		p.completeTask(task)
		p.logf("task_completed request=%s res=%s status=%s parsed=%d total=%d", shortRef(task.RequestId), shortRef(task.ResId), status, len(comments), len(list))
		p.sendCommentStatus(task, "completed", status, len(comments))
	} else {
		p.logf("comment_response_emitted source=strong_response_sign status=%s parsed=%d total=%d", status, len(comments), len(list))
	}
}

// extractComments 从 finderGetCommentDetail 的响应 data 中尽力提取评论列表。
// 视频号 web 接口结构可能调整，这里做多路径探测；提取不到时返回空数组，
// 前端可回退展示 raw 数据。
func extractComments(data map[string]interface{}) []CommentItem {
	if data == nil {
		return []CommentItem{}
	}

	var list []interface{}
	// commentInfo 为 FinderGetCommentList 的真实字段名，其余为兼容候选
	for _, key := range []string{"commentInfo", "commentList", "comments", "comment"} {
		if v, ok := data[key].([]interface{}); ok {
			list = v
			break
		}
	}
	if list == nil {
		// 兼容嵌套一层 object 的结构
		if obj, ok := data["object"].(map[string]interface{}); ok {
			for _, key := range []string{"commentInfo", "commentList", "comments"} {
				if v, ok := obj[key].([]interface{}); ok {
					list = v
					break
				}
			}
		}
	}
	if list == nil {
		return []CommentItem{}
	}

	items := make([]CommentItem, 0, len(list))
	for _, c := range list {
		m, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		item := CommentItem{}

		if v, ok := m["nickname"].(string); ok {
			item.NickName = v
		} else if v, ok := m["nickName"].(string); ok {
			item.NickName = v
		} else if author, ok := m["authorContact"].(map[string]interface{}); ok {
			if v, ok := author["nickname"].(string); ok {
				item.NickName = v
			}
		} else if author, ok := m["author"].(map[string]interface{}); ok {
			if v, ok := author["nickname"].(string); ok {
				item.NickName = v
			}
		}

		if v, ok := m["content"].(string); ok {
			item.Content = v
		} else if v, ok := m["text"].(string); ok {
			item.Content = v
		}

		for _, key := range []string{"likeCount", "likeCnt", "like_count"} {
			if v, ok := m[key].(float64); ok {
				item.LikeCount = v
				break
			}
		}
		// createtime 可能是字符串（如 "1785130902"）或数字
		for _, key := range []string{"createtime", "createTime", "createdAt"} {
			switch v := m[key].(type) {
			case float64:
				item.CreatedAt = v
			case string:
				if f, err := strconv.ParseFloat(v, 64); err == nil {
					item.CreatedAt = f
				}
			}
			if item.CreatedAt != 0 {
				break
			}
		}
		for _, key := range []string{"replyCount", "replyCnt", "expandCommentCount"} {
			if v, ok := m[key].(float64); ok {
				item.ReplyCnt = v
				break
			}
		}
		// IP 属地
		if ipr, ok := m["ipRegionInfo"].(map[string]interface{}); ok {
			if v, ok := ipr["regionText"].(string); ok {
				item.Region = v
			}
		}

		items = append(items, item)
	}
	return items
}
