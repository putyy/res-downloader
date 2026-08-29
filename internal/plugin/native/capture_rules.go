package native

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	shared "res-downloader/internal/model"
	"sort"
	"strconv"
	"strings"
)

const maxCaptureRules = 500

var genericBinaryMIMEs = []string{
	"application/octet-stream", "binary/octet-stream", "application/binary",
	"application/download", "application/x-download", "application/force-download", "application/x-force-download",
}

func DefaultResourceKinds() []shared.ResourceKindDefinition {
	return []shared.ResourceKindDefinition{
		resourceKind("media.image", "image", "#18a058", "图片", "Image"),
		resourceKind("media.audio", "audio", "#8a2be2", "音频", "Audio"),
		resourceKind("media.video", "video", "#2080f0", "视频", "Video"),
		resourceKind("stream.hls", "playlist", "#f0a020", "M3U8", "M3U8"),
		resourceKind("stream.live", "broadcast", "#d03050", "直播流", "Live stream"),
		resourceKind("stream.dash", "playlist", "#f0a020", "DASH 流", "DASH stream"),
		resourceKind("stream.binary", "file", "#606266", "二进制文件", "Binary file"),
		resourceKind("document.pdf", "document", "#d03050", "PDF", "PDF"),
		resourceKind("document.presentation", "document", "#f06b32", "演示文稿", "Presentation"),
		resourceKind("document.spreadsheet", "document", "#18a058", "表格", "Spreadsheet"),
		resourceKind("document.text", "document", "#2080f0", "文档", "Document"),
		resourceKind("font.webfont", "font", "#606266", "网页字体", "Web font"),
		resourceKind("file.archive", "archive", "#8f6b32", "压缩包", "Archive"),
	}
}

func resourceKind(id, icon, color, zh, en string) shared.ResourceKindDefinition {
	return shared.ResourceKindDefinition{
		ID: id, Icon: icon, Color: color,
		Locales: map[string]shared.PluginLocale{
			"zh-CN": {Name: zh}, "zh": {Name: zh}, "en": {Name: en},
		},
	}
}

func DefaultCaptureRules() []shared.CaptureRule {
	rules := []shared.CaptureRule{
		captureRule("image-png", []string{"image/png"}, "media.image", "image", ".png", "image", "http-file"),
		captureRule("image-apng", []string{"image/apng"}, "media.image", "image", ".apng", "image", "http-file"),
		captureRule("image-webp", []string{"image/webp"}, "media.image", "image", ".webp", "image", "http-file"),
		captureRule("image-jpeg", []string{"image/jpeg", "image/jpg"}, "media.image", "image", ".jpg", "image", "http-file"),
		captureRule("image-gif", []string{"image/gif"}, "media.image", "image", ".gif", "image", "http-file"),
		captureRule("image-avif", []string{"image/avif"}, "media.image", "image", ".avif", "image", "http-file"),
		captureRule("image-bitmap", []string{"image/bmp"}, "media.image", "image", ".bmp", "image", "http-file"),
		captureRule("image-tiff", []string{"image/tiff"}, "media.image", "image", ".tiff", "image", "http-file"),
		captureRule("image-heic", []string{"image/heic"}, "media.image", "image", ".heic", "image", "http-file"),
		captureRule("image-icon", []string{"image/x-icon"}, "media.image", "image", ".ico", "image", "http-file"),
		captureRule("image-svg", []string{"image/svg+xml"}, "media.image", "image", ".svg", "image", "http-file"),
		captureRule("image-photoshop", []string{"image/vnd.adobe.photoshop"}, "media.image", "image", ".psd", "", "http-file"),
		captureRule("image-jpeg2000", []string{"image/jp2", "image/jpeg2000"}, "media.image", "image", ".jp2", "image", "http-file"),

		captureRule("audio-mpeg", []string{"audio/mpeg", "audio/mp3"}, "media.audio", "audio", ".mp3", "audio", "http-file"),
		captureRule("audio-wave", []string{"audio/wav"}, "media.audio", "audio", ".wav", "audio", "http-file"),
		captureRule("audio-aiff", []string{"audio/aiff", "audio/x-aiff"}, "media.audio", "audio", ".aiff", "audio", "http-file"),
		captureRule("audio-aac", []string{"audio/aac"}, "media.audio", "audio", ".aac", "audio", "http-file"),
		captureRule("audio-ogg", []string{"audio/ogg"}, "media.audio", "audio", ".ogg", "audio", "http-file"),
		captureRule("audio-flac", []string{"audio/flac"}, "media.audio", "audio", ".flac", "audio", "http-file"),
		captureRule("audio-midi", []string{"audio/midi", "audio/x-midi"}, "media.audio", "audio", ".mid", "audio", "http-file"),
		captureRule("audio-wma", []string{"audio/x-ms-wma"}, "media.audio", "audio", ".wma", "audio", "http-file"),
		captureRule("audio-opus", []string{"audio/opus"}, "media.audio", "audio", ".opus", "audio", "http-file"),
		captureRule("audio-webm", []string{"audio/webm"}, "media.audio", "audio", ".webm", "audio", "http-file"),
		captureRule("audio-mp4", []string{"audio/mp4"}, "media.audio", "audio", ".m4a", "audio", "http-file"),
		captureRule("audio-amr", []string{"audio/amr"}, "media.audio", "audio", ".amr", "audio", "http-file"),

		captureRule("video-mp4", []string{"video/mp4"}, "media.video", "video", ".mp4", "video", "http-file"),
		captureRule("video-webm", []string{"video/webm"}, "media.video", "video", ".webm", "video", "http-file"),
		captureRule("video-ogg", []string{"video/ogg"}, "media.video", "video", ".ogv", "video", "http-file"),
		captureRule("video-avi", []string{"video/x-msvideo"}, "media.video", "video", ".avi", "video", "http-file"),
		captureRule("video-mpeg", []string{"video/mpeg"}, "media.video", "video", ".mpeg", "video", "http-file"),
		captureRule("video-quicktime", []string{"video/quicktime"}, "media.video", "video", ".mov", "video", "http-file"),
		captureRule("video-wmv", []string{"video/x-ms-wmv"}, "media.video", "video", ".wmv", "video", "http-file"),
		captureRule("video-3gpp", []string{"video/3gpp"}, "media.video", "video", ".3gp", "video", "http-file"),
		captureRule("video-matroska", []string{"video/x-matroska"}, "media.video", "video", ".mkv", "video", "http-file"),
		captureRule("stream-flv", []string{"video/x-flv", "audio/video"}, "stream.live", "video", ".flv", "video", "ffmpeg-hls"),
		captureRule("stream-hls", []string{"application/vnd.apple.mpegurl", "application/x-mpegurl", "audio/mpegurl", "audio/x-mpegurl"}, "stream.hls", "video", ".ts", "video", "hls"),
		{
			ID: "stream-hls-url", Name: "stream-hls-url", Enabled: true,
			Match: shared.CaptureRuleMatch{URL: []string{"*.m3u8", "*.m3u8?*"}, Status: []int{200, 206, 304}},
			Resource: shared.CaptureRuleResource{
				Kind: "stream.hls", Role: "video", Extension: ".ts", Executor: "hls",
				Capabilities:    []string{shared.ResourceCapabilityDownload, shared.ResourceCapabilityOpen, shared.ResourceCapabilityCopy, shared.ResourceCapabilityPreview},
				PreviewRenderer: "video", PreviewMode: "proxy",
			},
		},
		captureRule("stream-dash", []string{"application/dash+xml"}, "stream.dash", "manifest", ".mpd", "", "http-file"),

		captureRule("document-pdf", []string{"application/pdf"}, "document.pdf", "document", ".pdf", "pdf", "http-file"),
		captureRule("document-ppt", []string{"application/vnd.ms-powerpoint"}, "document.presentation", "document", ".ppt", "", "http-file"),
		captureRule("document-pptx", []string{"application/vnd.openxmlformats-officedocument.presentationml.presentation"}, "document.presentation", "document", ".pptx", "", "http-file"),
		captureRule("document-xls", []string{"application/vnd.ms-excel"}, "document.spreadsheet", "document", ".xls", "", "http-file"),
		captureRule("document-xlsx", []string{"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"}, "document.spreadsheet", "document", ".xlsx", "", "http-file"),
		captureRule("document-csv", []string{"text/csv"}, "document.spreadsheet", "document", ".csv", "text", "http-file"),
		captureRule("document-doc", []string{"application/msword"}, "document.text", "document", ".doc", "", "http-file"),
		captureRule("document-rtf", []string{"application/rtf", "text/rtf"}, "document.text", "document", ".rtf", "text", "http-file"),
		captureRule("document-odt", []string{"application/vnd.oasis.opendocument.text"}, "document.text", "document", ".odt", "", "http-file"),
		captureRule("document-docx", []string{"application/vnd.openxmlformats-officedocument.wordprocessingml.document"}, "document.text", "document", ".docx", "", "http-file"),
		captureRule("font-woff", []string{"font/woff"}, "font.webfont", "font", ".woff", "", "http-file"),
		captureRule("font-woff2", []string{"font/woff2"}, "font.webfont", "font", ".woff2", "", "http-file"),
		captureRule("font-ttf", []string{"font/ttf", "application/x-font-ttf"}, "font.webfont", "font", ".ttf", "", "http-file"),
		captureRule("font-otf", []string{"font/otf", "application/x-font-opentype"}, "font.webfont", "font", ".otf", "", "http-file"),
		captureRule("font-eot", []string{"application/vnd.ms-fontobject"}, "font.webfont", "font", ".eot", "", "http-file"),

		captureRule("archive-zip", []string{"application/zip", "application/x-zip-compressed"}, "file.archive", "archive", ".zip", "", "http-file"),
		captureRule("archive-rar", []string{"application/vnd.rar", "application/x-rar-compressed"}, "file.archive", "archive", ".rar", "", "http-file"),
		captureRule("archive-7z", []string{"application/x-7z-compressed"}, "file.archive", "archive", ".7z", "", "http-file"),
		captureRule("archive-tar", []string{"application/x-tar"}, "file.archive", "archive", ".tar", "", "http-file"),
		captureRule("archive-gzip", []string{"application/gzip", "application/x-gzip"}, "file.archive", "archive", ".gz", "", "http-file"),
		captureRule("archive-bzip2", []string{"application/x-bzip2"}, "file.archive", "archive", ".bz2", "", "http-file"),
		captureRule("archive-xz", []string{"application/x-xz"}, "file.archive", "archive", ".xz", "", "http-file"),

		captureRule("binary-file", genericBinaryMIMEs, "stream.binary", "binary", "", "", "http-file"),
	}
	for index := range rules {
		rules[index].Priority = len(rules) - index
	}
	return rules
}

type genericBinaryInference struct {
	ruleID string
}

// genericBinaryExtensions is deliberately limited to file types understood by
// the built-in detector. Extension inference is only used after a generic
// binary MIME matched stream.binary, so a specific server MIME always wins.
var genericBinaryExtensions = map[string]genericBinaryInference{
	".png":  {ruleID: "image-png"},
	".apng": {ruleID: "image-apng"},
	".webp": {ruleID: "image-webp"},
	".jpg":  {ruleID: "image-jpeg"},
	".jpeg": {ruleID: "image-jpeg"},
	".jpe":  {ruleID: "image-jpeg"},
	".jfif": {ruleID: "image-jpeg"},
	".gif":  {ruleID: "image-gif"},
	".avif": {ruleID: "image-avif"},
	".bmp":  {ruleID: "image-bitmap"},
	".tif":  {ruleID: "image-tiff"},
	".tiff": {ruleID: "image-tiff"},
	".heic": {ruleID: "image-heic"},
	".ico":  {ruleID: "image-icon"},
	".svg":  {ruleID: "image-svg"},
	".psd":  {ruleID: "image-photoshop"},
	".jp2":  {ruleID: "image-jpeg2000"},

	".mp3":  {ruleID: "audio-mpeg"},
	".wav":  {ruleID: "audio-wave"},
	".wave": {ruleID: "audio-wave"},
	".aif":  {ruleID: "audio-aiff"},
	".aiff": {ruleID: "audio-aiff"},
	".aifc": {ruleID: "audio-aiff"},
	".aac":  {ruleID: "audio-aac"},
	".ogg":  {ruleID: "audio-ogg"},
	".oga":  {ruleID: "audio-ogg"},
	".flac": {ruleID: "audio-flac"},
	".mid":  {ruleID: "audio-midi"},
	".midi": {ruleID: "audio-midi"},
	".wma":  {ruleID: "audio-wma"},
	".opus": {ruleID: "audio-opus"},
	".weba": {ruleID: "audio-webm"},
	".m4a":  {ruleID: "audio-mp4"},
	".m4b":  {ruleID: "audio-mp4"},
	".amr":  {ruleID: "audio-amr"},

	".mp4":  {ruleID: "video-mp4"},
	".m4v":  {ruleID: "video-mp4"},
	".webm": {ruleID: "video-webm"},
	".ogv":  {ruleID: "video-ogg"},
	".avi":  {ruleID: "video-avi"},
	".mpeg": {ruleID: "video-mpeg"},
	".mpg":  {ruleID: "video-mpeg"},
	".mpe":  {ruleID: "video-mpeg"},
	".mov":  {ruleID: "video-quicktime"},
	".qt":   {ruleID: "video-quicktime"},
	".wmv":  {ruleID: "video-wmv"},
	".3gp":  {ruleID: "video-3gpp"},
	".3gpp": {ruleID: "video-3gpp"},
	".mkv":  {ruleID: "video-matroska"},
	".flv":  {ruleID: "stream-flv"},

	".pdf":  {ruleID: "document-pdf"},
	".ppt":  {ruleID: "document-ppt"},
	".pptx": {ruleID: "document-pptx"},
	".xls":  {ruleID: "document-xls"},
	".xlsx": {ruleID: "document-xlsx"},
	".csv":  {ruleID: "document-csv"},
	".doc":  {ruleID: "document-doc"},
	".rtf":  {ruleID: "document-rtf"},
	".odt":  {ruleID: "document-odt"},
	".docx": {ruleID: "document-docx"},

	".woff":  {ruleID: "font-woff"},
	".woff2": {ruleID: "font-woff2"},
	".ttf":   {ruleID: "font-ttf"},
	".otf":   {ruleID: "font-otf"},
	".eot":   {ruleID: "font-eot"},

	".zip":  {ruleID: "archive-zip"},
	".rar":  {ruleID: "archive-rar"},
	".7z":   {ruleID: "archive-7z"},
	".tar":  {ruleID: "archive-tar"},
	".gz":   {ruleID: "archive-gzip"},
	".tgz":  {ruleID: "archive-gzip"},
	".bz2":  {ruleID: "archive-bzip2"},
	".tbz2": {ruleID: "archive-bzip2"},
	".xz":   {ruleID: "archive-xz"},
	".txz":  {ruleID: "archive-xz"},
}

func captureRule(id string, mime []string, kind, role, extension, renderer, executor string) shared.CaptureRule {
	capabilities := []string{shared.ResourceCapabilityDownload, shared.ResourceCapabilityOpen, shared.ResourceCapabilityCopy}
	if renderer != "" {
		capabilities = append(capabilities, shared.ResourceCapabilityPreview)
	}
	return shared.CaptureRule{
		ID: id, Name: id, Enabled: true,
		Match: shared.CaptureRuleMatch{MIME: mime, Status: []int{200, 206, 304}},
		Resource: shared.CaptureRuleResource{
			Kind: kind, Role: role, Extension: extension, Executor: executor,
			Capabilities: capabilities, PreviewRenderer: renderer, PreviewMode: "proxy",
		},
	}
}

func DecodeCaptureRules(value interface{}) ([]shared.CaptureRule, error) {
	if value == nil {
		return DefaultCaptureRules(), nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var rules []shared.CaptureRule
	if err := json.Unmarshal(raw, &rules); err != nil {
		return nil, err
	}
	if err := ValidateCaptureRules(rules); err != nil {
		return nil, err
	}
	return rules, nil
}

func ValidateCaptureRules(rules []shared.CaptureRule) error {
	raw, err := json.Marshal(rules)
	if err != nil {
		return err
	}
	if len(raw) > 4*1024*1024 {
		return errors.New("capture rules exceed 4 MiB")
	}
	if len(rules) > maxCaptureRules {
		return fmt.Errorf("capture rules exceed %d entries", maxCaptureRules)
	}
	ids := make(map[string]struct{}, len(rules))
	for index, rule := range rules {
		if len(rule.Name) > 256 {
			return fmt.Errorf("capture rule %q name is too long", rule.ID)
		}
		if rule.Priority < -1000000 || rule.Priority > 1000000 {
			return fmt.Errorf("capture rule %q priority is out of range", rule.ID)
		}
		if !captureIdentifier(rule.ID) {
			return fmt.Errorf("capture rule %d has invalid id %q", index, rule.ID)
		}
		if _, exists := ids[rule.ID]; exists {
			return fmt.Errorf("duplicate capture rule id %q", rule.ID)
		}
		ids[rule.ID] = struct{}{}
		if len(rule.Match.MIME) == 0 && len(rule.Match.URL) == 0 && len(rule.Match.ContentDisposition) == 0 {
			return fmt.Errorf("capture rule %q requires a MIME, URL or Content-Disposition matcher", rule.ID)
		}
		if len(rule.Match.MIME)+len(rule.Match.URL)+len(rule.Match.ContentDisposition) > 100 {
			return fmt.Errorf("capture rule %q has too many string matchers", rule.ID)
		}
		if rule.Match.MinSize < 0 || rule.Match.MaxSize < 0 || (rule.Match.MaxSize > 0 && rule.Match.MinSize > rule.Match.MaxSize) {
			return fmt.Errorf("capture rule %q has an invalid size range", rule.ID)
		}
		for _, status := range rule.Match.Status {
			if status < 100 || status > 599 {
				return fmt.Errorf("capture rule %q has invalid HTTP status %d", rule.ID, status)
			}
		}
		for _, pattern := range append(append(append([]string(nil), rule.Match.MIME...), rule.Match.URL...), rule.Match.ContentDisposition...) {
			if strings.TrimSpace(pattern) == "" || len(pattern) > 2048 {
				return fmt.Errorf("capture rule %q has an invalid empty or oversized matcher", rule.ID)
			}
		}
		if !captureIdentifier(rule.Resource.Kind) {
			return fmt.Errorf("capture rule %q has invalid resource kind %q", rule.ID, rule.Resource.Kind)
		}
		if rule.Resource.Role != "" && !captureIdentifier(rule.Resource.Role) {
			return fmt.Errorf("capture rule %q has invalid track role %q", rule.ID, rule.Resource.Role)
		}
		if rule.Resource.Executor != "" && rule.Resource.Executor != "http-file" && rule.Resource.Executor != "hls" && rule.Resource.Executor != "ffmpeg-hls" {
			return fmt.Errorf("capture rule %q has unsupported executor %q", rule.ID, rule.Resource.Executor)
		}
		if !captureExtension(rule.Resource.Extension) {
			return fmt.Errorf("capture rule %q has invalid extension %q", rule.ID, rule.Resource.Extension)
		}
		for _, capability := range rule.Resource.Capabilities {
			switch capability {
			case shared.ResourceCapabilityDownload, shared.ResourceCapabilityPreview, shared.ResourceCapabilityOpen, shared.ResourceCapabilityCopy:
			default:
				return fmt.Errorf("capture rule %q has unsupported capability %q", rule.ID, capability)
			}
		}
		if rule.Resource.PreviewRenderer != "" && !contains(rule.Resource.Capabilities, shared.ResourceCapabilityPreview) {
			return fmt.Errorf("capture rule %q preview renderer requires the preview capability", rule.ID)
		}
		if rule.Resource.PreviewRenderer != "" && !captureIdentifier(rule.Resource.PreviewRenderer) {
			return fmt.Errorf("capture rule %q has invalid preview renderer %q", rule.ID, rule.Resource.PreviewRenderer)
		}
	}
	return nil
}

func MatchCaptureRule(rules []shared.CaptureRule, observation shared.Observation) (*shared.CaptureRule, int64) {
	if observation.Response == nil {
		return nil, 0
	}
	sorted := append([]shared.CaptureRule(nil), rules...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Priority > sorted[j].Priority })
	size, _ := strconv.ParseInt(firstHeader(observation.Response.Headers, "Content-Length"), 10, 64)
	for index := range sorted {
		if sorted[index].Enabled && captureRuleMatches(sorted[index], observation, size) {
			return &sorted[index], size
		}
	}
	return nil, size
}

func inferGenericBinaryCaptureRule(rules []shared.CaptureRule, observation shared.Observation, size int64) (*shared.CaptureRule, string, string) {
	extension, source := observedFileExtension(observation)
	inference, exists := genericBinaryExtensions[extension]
	if !exists {
		return nil, "", ""
	}
	for index := range rules {
		if rules[index].Enabled && rules[index].ID == inference.ruleID {
			inferred := rules[index]
			canonicalMIME := captureRuleCanonicalMIME(inferred)
			inferred.Resource.Extension = extension
			inferred.Match.MIME = nil
			if !captureRuleMatches(inferred, observation, size) {
				return nil, "", ""
			}
			return &inferred, canonicalMIME, source
		}
	}
	return nil, "", ""
}

func isGenericBinaryMIME(value string) bool {
	value = strings.ToLower(strings.TrimSpace(strings.Split(value, ";")[0]))
	if value == "" {
		return true
	}
	return contains(genericBinaryMIMEs, value)
}

func observedFileExtension(observation shared.Observation) (string, string) {
	if observation.Response != nil {
		if title := captureTitle(observation.Response.Headers); title != "" {
			title = strings.ReplaceAll(title, "\\", "/")
			if extension := strings.ToLower(filepath.Ext(pathBase(title))); extension != "" {
				return extension, "content-disposition-extension"
			}
		}
	}
	parsed, err := url.Parse(observation.Request.URL)
	if err != nil {
		return "", ""
	}
	if extension := strings.ToLower(filepath.Ext(filepath.Base(parsed.Path))); extension != "" {
		return extension, "url-extension"
	}
	return "", ""
}

func pathBase(value string) string {
	if index := strings.LastIndex(value, "/"); index >= 0 {
		return value[index+1:]
	}
	return value
}

func captureRuleCanonicalMIME(rule shared.CaptureRule) string {
	for _, value := range rule.Match.MIME {
		value = strings.ToLower(strings.TrimSpace(strings.Split(value, ";")[0]))
		if value != "" && !strings.Contains(value, "*") {
			return value
		}
	}
	return ""
}

func captureRuleMatches(rule shared.CaptureRule, observation shared.Observation, size int64) bool {
	match := rule.Match
	if len(match.Status) > 0 && !containsInt(match.Status, observation.Response.StatusCode) {
		return false
	}
	if (match.MinSize > 0 || match.MaxSize > 0) && size <= 0 {
		return false
	}
	if match.MinSize > 0 && size < match.MinSize {
		return false
	}
	if match.MaxSize > 0 && size > match.MaxSize {
		return false
	}
	mime := strings.ToLower(strings.TrimSpace(strings.Split(observation.Response.ContentType, ";")[0]))
	if len(match.MIME) > 0 && !matchesAnyPattern(match.MIME, mime) {
		return false
	}
	if len(match.URL) > 0 && !matchesAnyPattern(match.URL, observation.Request.URL) && !matchesAnyPattern(match.URL, observation.Request.Path) {
		return false
	}
	disposition := firstHeader(observation.Response.Headers, "Content-Disposition")
	if len(match.ContentDisposition) > 0 && !matchesAnyPattern(match.ContentDisposition, disposition) {
		return false
	}
	return true
}

func CaptureRuleExtension(rule shared.CaptureRule, rawURL string) string {
	if rule.Resource.Extension != "" {
		return rule.Resource.Extension
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	extension := filepath.Ext(filepath.Base(parsed.Path))
	if captureExtension(extension) {
		return extension
	}
	return ""
}

func matchesAnyPattern(patterns []string, value string) bool {
	value = strings.ToLower(value)
	for _, pattern := range patterns {
		if captureWildcardMatch(strings.ToLower(strings.TrimSpace(pattern)), value) {
			return true
		}
	}
	return false
}

func captureWildcardMatch(pattern, value string) bool {
	if pattern == "*" {
		return true
	}
	parts := strings.Split(pattern, "*")
	position := 0
	for index, part := range parts {
		if part == "" {
			continue
		}
		found := strings.Index(value[position:], part)
		if found < 0 || (index == 0 && !strings.HasPrefix(pattern, "*") && found != 0) {
			return false
		}
		position += found + len(part)
	}
	return strings.HasSuffix(pattern, "*") || strings.HasSuffix(value, parts[len(parts)-1])
}

func captureIdentifier(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for _, char := range value {
		if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') && (char < '0' || char > '9') && char != '.' && char != '-' && char != '_' {
			return false
		}
	}
	return true
}

func captureExtension(extension string) bool {
	if extension == "" {
		return true
	}
	if len(extension) < 2 || len(extension) > 20 || extension[0] != '.' {
		return false
	}
	for _, char := range extension[1:] {
		if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') && (char < '0' || char > '9') && char != '.' && char != '-' && char != '_' {
			return false
		}
	}
	return true
}

func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func containsInt(values []int, expected int) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
