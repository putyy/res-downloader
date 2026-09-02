# 微信视频号"获取评论"功能 — 完整上下文档案

> **文档目的**：记录该功能从设计到多轮调试的全部上下文：架构、逆向结论、每次失败的根因、当前代码状态、未验证的最后一版方案与后续排查手册。
> **状态**：微信 4.1.11 的注入、实际视频自动关联、完整三元组请求、空评论与 25 条有评论结果均已通过真实环境终验；仍需再用一个不同的有评论动态做原生内容对照，确认跨目标结果一致性。

---

## 1. 目标

在 res-downloader（Wails 桌面应用，Go 后端 + Vue3 前端，本地 MITM 代理嗅探）的资源列表中，为每个微信视频号资源增加"获取评论"按钮，点击后展示**该视频自己的**评论列表（昵称/IP 属地/内容/点赞数），并做历史记录。

## 2. 既有实现（已验证可用的部分）

| 模块 | 位置 | 状态 |
|---|---|---|
| "获取评论"按钮（视频号资源操作菜单） | `frontend/src/components/Action.vue` | ✅ 可用 |
| `POST /api/get-comments` 端点 | `core/http.go` / `core/middleware.go` | ✅ 可用 |
| 评论任务队列（入队→页面轮询取走） | `QqPlugin.AddCommentTask/popCommentTasks` | ✅ 可用 |
| 页面注入 JS 轮询器（3s，type=3） | `injectHooks()` in `core/plugins/plugin.qq.com.go` | ✅ 可用 |
| 评论解析（`commentInfo` 结构，含 IP 属地） | `extractComments()` | ✅ 单元测试覆盖 |
| 评论弹窗 + 仅在显式请求时弹出 | `frontend/src/views/index.vue` | ✅ 可用 |
| 历史记录（挂载资源 + localStorage 持久化，二次点击直显） | 同上 | ✅ 可用 |
| 诊断日志（`[QqPlugin]` 前缀） | `~/Library/Preferences/res-downloader/logs/app.log` | ✅ |
| 单元测试（原有 7 个均保留，当前插件列出 20 个，含真实 bundle 注入、feed 模型自动关联、权威目标与去重测试） | `core/plugins/plugin_*_test.go` + `testdata/wxjs_sample.js` | ✅ 19 通过；外部 bundle 用例未指定路径时按预期跳过 |

**回传通道**（注入 JS ↔ 代理拦截伪 URL `https://wxapp.tc.qq.com/res-downloader/wechat?type=N`）：

| type | 方向 | 用途 |
|---|---|---|
| 1 | 页面→Go | `get media()` 回传 objectDesc（嗅探主通道） |
| 2 | 页面→Go | detail 钩子回传 objectDesc（WxAction=false 时） |
| 3 | 页面←Go | 轮询取评论任务 |
| 4 | 页面→Go | 评论/detail 响应数据 |
| 5 | 页面→Go | 执行器失败原因上报 |
| 6 | 页面→Go | 评论区打开时的 nonce+当前 objectDesc（**已被证实不可靠**） |
| 7 | 页面→Go | 执行器 detail 换得 nonce 后的 oid↔nonce 配对 |
| 8 | 页面→Go | 推荐列表（旧 method 级钩子，**正则不匹配从未触发**） |
| 9 | 页面→Go | **post 总出口拦截**（推荐/评论列表/评论详情的解码后数据 + 完整参数） |

## 3. 逆向结论（全部来自真实 bundle 与线上日志，可靠）

当前 bundle：`res.wx.qq.com/t/wx_fed/finder/web/web-finder/res/js/virtual_svg-icons-register.publish<hash>.js`（448KB，已存 `core/plugins/testdata/wxjs_sample.js`）

1. **接口三件套**：`FinderGetCommentList`（评论面板）、`FinderGetCommentDetail`（刷新视频信息，`needObject:1`）、`FinderGetRecommend`（feed 列表）。
2. **所有接口汇聚到 `this.post({name, data})`**（两个 API 客户端类，各一份 `async post(n){try{return await this.invoke(n)}catch...}`）。钩 post = 拿到全部解码后 JSON + 合并完毕的最终参数。**这是最佳拦截点**。
3. **服务端按 `finderBasereq.objectBaseInfos.sessionBuffer` 定位视频**，不是 objectNonceId、更不是 objectid。铁证：同一 nonce 两次请求返回不同视频的评论（15:41:25 vs 15:41:38 日志）。
4. **feed 对象自带三元组**：`{id, objectNonceId, sessionBuffer}`（bundle 中 feed model 有 `sessionBuffer` 字段；`toggleLikeFeed` 调用即 `{objectid, objectNonceId, sessionBuffer}`）。`FinderGetRecommend` 响应是**批量、同源、随会话新鲜**的身份源。
5. `objectDesc` 顶层**没有 id/nonceId 字段**（只有 media、description、topic 等 ~17 个）。
6. 组件 `this.id`（≥10 位纯数字）是可靠的 objectId；组件扫描的 nonceId **会因 Vue 列表组件复用而错配**（陈旧值），且 nonceId 数字前缀 ≠ objectId。
7. 手动拉评论的方法级参数只有 `{objectNonceId, direction:2}`；post 层完整参数含 `finderBasereq.objectBaseInfos[0].sessionBuffer`。
8. 评论响应结构：`data.commentInfo[]`，字段 `nickname`（或 `authorContact.nickname`）、`content`、`likeCount`、`createtime`（**字符串**）、`expandCommentCount`（回复数）、`ipRegionInfo.regionText`、楼中楼 `levelTwoComment`。
9. 客户端 webview 缓存极顽固：注入 JS URL 已加启动时间戳（`injectNonce`），但**客户端必须 Cmd+Q 重开**才能拿到新注入。

## 4. 失败时间线与根因（每轮都改变了方案）

| 轮次 | 现象 | 根因 |
|---|---|---|
| 1 | 按钮不显示 | 旧缓存资源无 `wx_object_id`；objectDesc 无 id 字段 |
| 2 | 缺 ID | nonceId 前缀 ≠ objectId（实测证伪） |
| 3 | 超时 | detail fn 引用未捕获 + 页面缓存旧 JS（`wrapped=false`） |
| 4 | 找不到评论函数 | 评论区面板走 `FinderGetCommentList`，钩的是 Detail |
| 5 | 串视频 | 组件扫描 nonceId 错配（Vue 复用陈旧值） |
| 6 | 全部"未获取身份" | detail 接口在用户客户端**从不触发**，学习断供 |
| 7 | 全是当前视频评论 | type=6"当前媒体+nonce"关联竞态 |
| 8 | 依然全同 | `finderGetRecommend` 钩子正则不匹配（方法体非 `return this.post` 开头）→ 身份断供回退 type=6 |
| 9 | 依然全同 | **sessionBuffer 才是服务端定位键**，模板复用把当前视频的 sessionBuffer 带进每个请求 |

## 5. 当前方案（第 9 轮，已实现未验证）

- post 总出口钩子持续拦截 `FinderGetRecommend`，三元组 `{objectId, objectNonceId, sessionBuffer}` 按 `urlSign`（媒体 URL 的 Md5）批量入库（日志：`post hook recommend: learned N feed identities`）。
- 点 💬 → 按 urlSign 取完整三件套 → 执行器构造 `{objectid, objectNonceId, sessionBuffer, direction:2}` 并把 `finderBasereq.objectBaseInfos` 替换为 `[{sessionBuffer: 目标视频自己的}]` → 调 list。
- 回传保险丝：主动请求用本地 `requestId` 关联；后端复核三元组与两层 sessionBuffer，前端再检查目标签名。有效空评论作为 `no_comments` 完成态展示，不再误报超时。

**2026-08-29 本地加固**：真实日志确认历史主动任务从未出现 `hasSB=true`，因此此前实验没有实际进入完整第 9 版。当前实现改为仅允许同一个 recommend feed 原子取得的完整三元组入队；不完整身份立即返回 `comment_identity_unavailable`，不再用 type=6、组件 nonce、detail 或无 SB 模板降级调用。主动任务增加 `requestId`，post 回传后校验 objectId、nonce、顶层及嵌套 sessionBuffer；空评论、部分解析、目标错配、请求失败和超时分层。真实定位正确性仍未验证。

**2026-08-30 入口修复**：移除按钮对 `wx_object_id` / `DecodeKey` 的旧显示依赖。新资源由后端写入非敏感的 `wx_channel=1` 来源标记，旧缓存资源按 `Domain=qq.com` 且 URL host 为 `finder.video.qq.com` 兼容识别。该标记只控制入口展示，不能作为目标身份；点击后的任务仍必须通过 recommend feed 完整三元组门槛。

**2026-08-30 可发现性补强**：真实 Wails 窗口在本地资源缓存为空时只显示“无数据”，资源行级入口没有挂载点。现增加顶部常驻“获取评论”入口：无候选时只展示抓取指引；一个视频号资源时进入原流程；多个资源时必须先勾选一个。该入口不缓存、推断或绕过目标身份。

**2026-09-02 微信 4.1.11 兼容修复**：从客户端 Chromium 缓存还原最新 bundle `virtual_svg-icons-register.publishCwe-fFAJiv.js`（SHA-256 `b5dd6b7a…398ae76`）。`FinderGetRecommend`、`FinderGetCommentList`、`FinderGetCommentDetail`、两处 `post -> invoke` 和 `sessionBuffer` 模型仍存在；旧 `qqPostRegex` 因硬编码 `Ce.JSAPI_UNKNOWN`，而新版改为 `JstransferErr.JSAPI_UNKNOWN`，导致 post 总出口 0 命中。现改为仅替换 `post -> invoke` 稳定前缀并原样保留客户端 catch，同时记录四类钩子命中数和 `post_shape_unmatched` 诊断。真实新版 bundle 的静态测试、经本地代理的 identity 与 Brotli 请求均命中 `media/detail/list=1、post=2`，注入后整包通过 `node --check`。

同次升级还观测到媒体 CDN 从 `finder.video.qq.com` 扩展为 `findera4.video.qq.com:443`。旧后端因此未进入 QQ 专用插件，`GetTopLevelDomain` 又把端口保留成 `qq.com:443`；前端精确主机判断进一步隐藏了行内评论入口。现统一限定为 `finder*.video.qq.com`，后端在完整 URL和 `Request.Host` 两种输入上移除端口，前端兼容已缓存的 `qq.com:443`。该规则仍不接受普通 `video.qq.com` 或伪造后缀。

**2026-09-02 真实链路终验补充**：微信 4.1.11 的 `FinderGetRecommend` 中，当前样本的 `objectDesc.media` 只提供 `mediaType=9` 的 20304 封面资源；页面后续播放的 20302 视频 URL 与之不相同。对 203 个推荐 objectId 和 81 个页面媒体钩子 objectId 做脱敏哈希交叉核验，交集为 0，故不能用组件扫描或跨 CDN/描述启发式把真实视频 URL 反向绑定到三元组。当前实现直接把同一 recommend feed 的封面作为“评论目标载体”，保持图片分类并写入 `wx_comment_target=1`；该载体绕过媒体类型抓取过滤，但下载语义不伪装成视频。其本地 `urlSign` 只负责选择，微信服务端定位仍只使用同源 `{objectId, objectNonceId, sessionBuffer}`。

最终实测记录：`hook_injected media/detail/list=1/1/1 post=2`；首批 `feeds=60 complete=60 missingSB=0`；权威目标稳定出现 `media_emitted source=post_recommend`，重复批次出现 `authoritative_duplicate`。对其中一条目标调用 `/api/get-comments` 后，完整时间线为 `task_queued hasTriple=true → task_picked → request_verified → task_completed status=no_comments`，Wails 弹窗显示“目标已校验”“共 0 条”“这是一次有效的空结果，不是超时”。这证明注入、选目标、任务轮询、两层 sessionBuffer 校验、回传解析和 UI 展示全链路可用；由于抽中的动态确实返回 0 条，尚未完成“两个有评论动态与微信原生内容逐项一致”的验证。

同次实测发现 macOS 15 上 `networksetup -setwebproxy/-setsecurewebproxy` 可能只写入 server/port 而保持 `Enabled: No`，导致应用按钮与真实系统代理状态不一致。`SystemSetup.setProxy` 已补充显式 `-setwebproxystate on/-setsecurewebproxystate on`，并改为单个网络服务的全部命令成功后才报告成功。最终二进制通过本地 API 单次验证：内部状态与 `scutil` 从 `false/0` 同步切换到 `true/1`，关闭后同步回 `false/0`；SOCKS 配置未改变。

**若此版仍失败，下一步排查方向**：
1. 日志对比：同资源入队时的 sessionBuffer 与手动展开该视频评论区时 post 层捕获的 sessionBuffer 是否一致。
2. 可能遗漏的字段：`identityScene`、`scene`、`pullScene`、`finderUsername`（模板已带），或服务端还校验 `lastBuffer` 链。
3. 检查 recommend 响应中 sessionBuffer 是否"上送用"而非"下载用"（上送的 objectBaseInfos 是客户端已浏览上下文，评论接口可能要的是另一种 buffer）。
4. 终极对照实验：对同一视频，抓手动拉评论的完整 post 参数（type=9 arg 已有），与我们主动构造的参数做 diff，缺的字段一次补齐。

**2026-09-02 实际视频自动关联修复**：继续实测发现，`identity_unavailable` 发生在普通 20302 视频行，而完整三元组只绑定在 recommend 的 20304 封面行。复核最新 bundle 后确认：唯一一个 `get media()` 属于 feed 模型类，该类的同一实例直接持有 `id`、`objectNonceId`、`sessionBuffer`、`objectDesc`；此前注入脚本只回传前两项，还执行了不必要的通用字段扫描。现改为从该模型的精确字段原子回传三元组，并把实际视频 `urlSign` 映射为 `source=feed_model`；三项缺一或没有明确来源标记时仍拒绝学习。`post` 总出口在首次 recommend 时同时保存该 API 客户端的原生 `finderGetCommentList` 方法，自动任务无需用户预先展开评论区。

真实终验：实际视频“暴躁主厨第二弹”出现 `media_identity_linked` 后，以其真实视频资源 id/urlSign 创建任务，日志依次为 `task_queued source=feed_model → task_picked → request_verified → task_completed status=success parsed=25 total=25`，Wails 评论弹窗显示目标已校验及 25 条评论。这证明普通视频行与完整身份的自动关联、原生 list 调用器提前捕获、请求校验、解析和 UI 展示均已打通。仍需用另一条明确有评论动态与微信原生内容做跨目标对照；分页仍不在本次范围内。

## 6. 调试手册

```bash
# 日志（[QqPlugin] 前缀为诊断埋点）
grep "QqPlugin" ~/Library/Preferences/res-downloader/logs/app.log | tail -30

# 关键日志行含义
# injecting hooks into ...        JS bundle 注入成功
# post hook recommend: learned N  推荐列表身份入库数（0 = 结构变了，会打印 data keys）
# identity_batch ... complete=N   post 推荐身份学习结果（complete 必须大于 0）
# task_queued ... hasTriple=true  完整三元组任务入队（字段仅记录长度与短哈希）
# task_picked                     页面轮询取走任务
# request_verified                post 回传参数通过三元组与两层 sessionBuffer 校验
# task_completed                  评论解析与前端事件发送完成
# task_failed code=...            字段缺失、错配、请求或解析失败（不记录完整参数/评论）

# 构建
cd frontend && npm run build && cd .. && ~/go/bin/wails build
# 产物：build/bin/res-downloader.app；前端 dev：wails dev

# 测试
go test ./core/plugins/ -v          # 原有 7 个测试不得删除；当前共 14 个
go test ./core/shared/ -v
cd frontend && npm test
node --check /tmp/wxjs_injected.js  # 注入后整包语法复核（测试自动生成）
```

**复现前提**：CA 证书已装、用 `scutil --proxy` 确认 HTTP/HTTPS 代理实际开启、**微信 Cmd+Q 重开**、进入视频号；页面初始评论列表调用会捕获函数引用。

## 8. 2026-08-29 本地验证记录

- `go test ./core/plugins/ -v`：原有 7 个与新增 4 个测试全部通过。
- `go test -race ./core/plugins/`：通过；同时移除了 `handleMedia` 中冗余的二次 goroutine。
- `node --check /tmp/wxjs_injected.js`：通过；真实样本中的 post 总出口固定命中 2 处。
- `frontend npm run build`：Vue 类型检查与生产构建通过。
- `wails build`：darwin/arm64 应用打包通过。
- 浏览器级测试：历史查看、目标校验、活动任务禁用、Enter/Esc、390px 窄屏和 40px 操作目标通过。
- 尚未执行：真实微信中两个不同视频的原生评论对照。只有出现 `task_queued hasTriple=true → task_picked → request_verified → task_completed` 后，才算真正验证第 9 版。

## 9. 2026-09-02 微信 4.1.11 验证记录

- 当前插件 14 个测试、shared 端口规范化测试、前端资源识别 3 个测试全部通过；插件与 shared 竞态检测通过。
- 对客户端缓存还原的最新完整 bundle 执行注入测试：四类钩子命中 `1/1/1/2`，注入后 `node --check` 通过。
- 通过运行中的本地代理分别请求 identity 与 Brotli 版本的线上 bundle：两次均记录 `hook_injected version=post-prefix-v2 ... post_matches=2`，交付脚本均含两个 `type=9` 钩子且语法有效。
- 前端生产构建和 Wails darwin/arm64 打包通过；新产物中顶部入口以及旧缓存 `qq.com:443` 资源的行内“获取评论”入口均已人工确认。
- 用户明确授权后完成系统代理与真实微信抓取。权威目标产生、完整三元组入队、页面轮询、主动请求校验、空评论完成态和结果弹窗均已通过；未发生 `identity_unavailable`、超时或目标错配。
- 仍需选择两个明确有评论的动态，对比应用结果与微信原生首屏昵称/内容，并确认两条动态互不串数据；这一项是剩余的真实性对照，不影响已验证的协议与状态链路。

## 7. 已知限制与合规提示

- 评论数据只有在页面产生过对应 API 调用（或主动代调）后才能获得；无法拉取分页（lastBuffer 链未实现）。
- 客户端 webview 缓存导致注入版本管理脆弱，`injectNonce` 只解决网页端。
- 评论含他人昵称/内容，采集与展示请注意合规边界。
