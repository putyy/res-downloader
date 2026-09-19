package naming

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	shared "res-downloader/internal/model"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	defaultFilenameTemplate = "{{title|default:resource|sanitize|truncate:80}}_{{date:20060102_150405}}.{{ext}}"
	maxFilenameTemplateSize = 4096
	// Keep room below the common 255-byte component limit for conflict suffixes
	// such as "(1)" and for filesystem-specific normalization.
	MaxFilenameSegmentBytes = 240
)

func RenderResourcePath(directory, template string, resource shared.ResourceCandidate, plan shared.DownloadPlan, now time.Time) (string, error) {
	if err := ValidateFilenameTemplate(template); err != nil {
		return "", err
	}
	if template == "" {
		template = defaultFilenameTemplate
	}
	track := selectedFilenameTrack(resource, plan)
	extension := strings.TrimPrefix(plan.Output.Extension, ".")
	variables := map[string]string{
		"title":   resource.Title,
		"id":      resource.ID,
		"kind":    resource.Kind,
		"plugin":  resource.Source.PluginID,
		"host":    shared.GetTopLevelDomain(track.URL),
		"track":   track.ID,
		"quality": track.Quality,
		"width":   positiveIntegerString(track.Width),
		"height":  positiveIntegerString(track.Height),
		"bitrate": positiveInt64String(track.Bitrate),
		"ext":     extension,
		"date":    now.Format("20060102"),
		"time":    now.Format("150405"),
	}
	if author := metadataString(resource.Metadata, "author"); author != "" {
		variables["author"] = author
	}
	rendered, err := ExpandFilenameTemplate(template, variables, resource.Metadata, now)
	if err != nil {
		return "", err
	}
	relative, err := safeRelativeResourcePath(rendered)
	if err != nil {
		return "", err
	}
	if extension != "" && !strings.EqualFold(filepath.Ext(relative), "."+extension) {
		relative += "." + SanitizeFilenameSegment(extension)
	}
	relative = truncateRelativePathSegments(relative)
	target := filepath.Join(directory, filepath.FromSlash(relative))
	root, err := filepath.Abs(directory)
	if err != nil {
		return "", err
	}
	target, err = filepath.Abs(target)
	if err != nil {
		return "", err
	}
	relativeTarget, err := filepath.Rel(root, target)
	if err != nil || relativeTarget == ".." || filepath.IsAbs(relativeTarget) || strings.HasPrefix(relativeTarget, ".."+string(filepath.Separator)) {
		return "", errors.New("filename template escaped the download directory")
	}
	return target, nil
}

// ValidateFilenameTemplate catches errors that do not depend on a particular
// resource. Values available only at download time are checked by RenderResourcePath.
func ValidateFilenameTemplate(template string) error {
	if template == "" {
		template = defaultFilenameTemplate
	}
	if len(template) > maxFilenameTemplateSize {
		return fmt.Errorf("filename template exceeds %d bytes", maxFilenameTemplateSize)
	}

	now := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	variables := map[string]string{"ext": "mp4", "date": "20260102", "time": "030405"}
	metadata := map[string]interface{}{
		"createdAt": now.UnixMilli(), "publishedAt": now.UnixMilli(),
	}
	var fallbackPath strings.Builder
	for remaining := template; ; {
		start := strings.Index(remaining, "{{")
		if start < 0 {
			fallbackPath.WriteString(remaining)
			break
		}
		fallbackPath.WriteString(remaining[:start])
		remaining = remaining[start+2:]
		end := strings.Index(remaining, "}}")
		if end < 0 {
			return errors.New("filename template contains an unclosed variable")
		}
		expression := strings.TrimSpace(remaining[:end])
		remaining = remaining[end+2:]
		variable, _, _ := strings.Cut(strings.TrimSpace(strings.Split(expression, "|")[0]), ":")
		if strings.HasPrefix(variable, "meta.") {
			key := strings.TrimPrefix(variable, "meta.")
			if key != "createdAt" && key != "publishedAt" {
				metadata[key] = "resource"
			}
		} else if variable != "ext" && variable != "date" && variable != "time" {
			variables[variable] = "resource"
		}
		optionalValue, err := evaluateFilenameExpression(expression, nil, nil, now)
		if err != nil {
			return err
		}
		// Keep the surrounding path when checking defaults. An expression may
		// start with a separator even though the complete path is relative.
		if optionalValue == "" {
			optionalValue, err = evaluateFilenameExpression(expression, variables, metadata, now)
			if err != nil {
				return err
			}
		}
		fallbackPath.WriteString(optionalValue)
	}

	rendered, err := ExpandFilenameTemplate(template, variables, metadata, now)
	if err != nil {
		return err
	}
	for _, value := range []string{rendered, fallbackPath.String()} {
		if isDriveQualifiedFilenamePath(value) {
			return errors.New("filename template must produce a relative path")
		}
		if _, err := safeRelativeResourcePath(value); err != nil {
			return err
		}
	}
	return nil
}

func isDriveQualifiedFilenamePath(value string) bool {
	return len(value) >= 2 && value[1] == ':' &&
		(value[0] >= 'A' && value[0] <= 'Z' || value[0] >= 'a' && value[0] <= 'z')
}

func rejectUnsafeFilenamePath(value string) error {
	value = strings.ReplaceAll(value, "\\", "/")
	if strings.HasPrefix(value, "/") {
		return errors.New("filename template must produce a relative path")
	}
	for _, part := range strings.Split(value, "/") {
		if strings.TrimSpace(part) == ".." {
			return errors.New("filename template must not contain parent path segments")
		}
	}
	return nil
}

func ExpandFilenameTemplate(template string, variables map[string]string, metadata map[string]interface{}, now time.Time) (string, error) {
	var output strings.Builder
	for len(template) > 0 {
		start := strings.Index(template, "{{")
		if start < 0 {
			output.WriteString(template)
			break
		}
		output.WriteString(template[:start])
		template = template[start+2:]
		end := strings.Index(template, "}}")
		if end < 0 {
			return "", errors.New("filename template contains an unclosed variable")
		}
		expression := strings.TrimSpace(template[:end])
		template = template[end+2:]
		value, err := evaluateFilenameExpression(expression, variables, metadata, now)
		if err != nil {
			return "", err
		}
		output.WriteString(value)
	}
	return output.String(), nil
}

func evaluateFilenameExpression(expression string, variables map[string]string, metadata map[string]interface{}, now time.Time) (string, error) {
	parts := strings.Split(expression, "|")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		return "", errors.New("filename template contains an empty variable")
	}
	variable, variableArgument, _ := strings.Cut(strings.TrimSpace(parts[0]), ":")
	var value string
	if variable == "date" && variableArgument != "" {
		value = now.Format(variableArgument)
	} else if variable == "time" && variableArgument != "" {
		value = now.Format(variableArgument)
	} else if variable == "created_at" || variable == "published_at" {
		key := "createdAt"
		if variable == "published_at" {
			key = "publishedAt"
		}
		value = formatMetadataTimestamp(metadata[key], variableArgument, now.Location())
	} else if strings.HasPrefix(variable, "meta.") {
		value = metadataPathString(metadata, strings.TrimPrefix(variable, "meta."))
	} else {
		value = variables[variable]
	}
	for _, rawFilter := range parts[1:] {
		filter, argument, _ := strings.Cut(strings.TrimSpace(rawFilter), ":")
		switch filter {
		case "sanitize":
			value = SanitizeFilenameSegment(value)
		case "truncate":
			limit, err := strconv.Atoi(argument)
			if err != nil || limit <= 0 || limit > 1000 {
				return "", fmt.Errorf("filename truncate filter has invalid limit %q", argument)
			}
			value = TruncateRunes(value, limit)
		case "default":
			if strings.TrimSpace(value) == "" {
				value = argument
			}
		case "prefix", "suffix":
			if strings.TrimSpace(value) == "" {
				value = ""
			} else if filter == "prefix" {
				value = argument + value
			} else {
				value += argument
			}
		case "lower":
			value = strings.ToLower(value)
		case "upper":
			value = strings.ToUpper(value)
		case "":
			return "", errors.New("filename template contains an empty filter")
		default:
			return "", fmt.Errorf("unsupported filename filter %q", filter)
		}
	}
	return value, nil
}

// Metadata timestamps are positive integer Unix milliseconds, not strings or
// seconds. Invalid optional metadata must not prevent a resource from downloading.
func formatMetadataTimestamp(raw interface{}, layout string, location *time.Location) string {
	var milliseconds float64
	switch value := raw.(type) {
	case int:
		milliseconds = float64(value)
	case int64:
		milliseconds = float64(value)
	case float64:
		milliseconds = value
	default:
		return ""
	}
	// Limit dates to year 9999 and keep all accepted integers exactly representable
	// in JSON/JavaScript numbers. This also rejects NaN, infinity and fractions.
	if math.IsNaN(milliseconds) || milliseconds <= 0 || milliseconds > 253402300799999 || math.Trunc(milliseconds) != milliseconds {
		return ""
	}
	if layout == "" {
		layout = "20060102"
	}
	timestamp := time.UnixMilli(int64(milliseconds)).In(location)
	if timestamp.Year() < 1 || timestamp.Year() > 9999 {
		return ""
	}
	return timestamp.Format(layout)
}

func selectedFilenameTrack(resource shared.ResourceCandidate, plan shared.DownloadPlan) shared.ResourceTrack {
	selectedInput := plan.Output.Input
	for _, input := range plan.Inputs {
		if input.ID != selectedInput {
			continue
		}
		for _, track := range resource.Tracks {
			if track.ID == input.ID {
				return track
			}
		}
		return shared.ResourceTrack{ID: input.ID, URL: input.URL, Extension: input.Extension}
	}
	if primary := shared.PrimaryResourceTrack(resource.Tracks); primary != nil {
		return *primary
	}
	if len(plan.Inputs) > 0 {
		return shared.ResourceTrack{ID: plan.Inputs[0].ID, URL: plan.Inputs[0].URL, Extension: plan.Inputs[0].Extension}
	}
	return shared.ResourceTrack{}
}

func safeRelativeResourcePath(value string) (string, error) {
	value = strings.ReplaceAll(value, "\\", "/")
	if err := rejectUnsafeFilenamePath(value); err != nil {
		return "", err
	}
	rawParts := strings.Split(value, "/")
	parts := make([]string, 0, len(rawParts))
	for _, rawPart := range rawParts {
		part := strings.TrimSpace(rawPart)
		if part == "" || part == "." {
			continue
		}
		part = TruncateFilenameSegment(SanitizeFilenameSegment(part), MaxFilenameSegmentBytes)
		if part == "" {
			part = "resource"
		}
		parts = append(parts, part)
	}
	if len(parts) == 0 {
		return "", errors.New("filename template produced an empty path")
	}
	return strings.Join(parts, "/"), nil
}

func SanitizeFilenameSegment(value string) string {
	value = strings.Map(func(char rune) rune {
		if unicode.IsControl(char) || strings.ContainsRune(`<>:"/\|?*＜＞：＂／＼｜？＊`, char) {
			return '_'
		}
		return char
	}, strings.TrimSpace(value))
	value = strings.TrimRight(value, ". ")
	upper := strings.ToUpper(value)
	reserved := map[string]bool{
		"CON": true, "PRN": true, "AUX": true, "NUL": true,
		"COM1": true, "COM2": true, "COM3": true, "COM4": true, "COM5": true, "COM6": true, "COM7": true, "COM8": true, "COM9": true,
		"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true, "LPT5": true, "LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
	}
	base := upper
	if index := strings.IndexByte(base, '.'); index >= 0 {
		base = base[:index]
	}
	if reserved[base] {
		value = "_" + value
	}
	return value
}

// TruncateFilenameSegment limits a path component by UTF-8 bytes without
// splitting a rune. A recognizable extension is retained when possible.
func TruncateFilenameSegment(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		if limit <= 0 {
			return ""
		}
		return value
	}
	extension := filepath.Ext(value)
	if extension == "" || len(extension) >= limit {
		return truncateUTF8Bytes(value, limit)
	}
	base := strings.TrimSuffix(value, extension)
	return truncateUTF8Bytes(base, limit-len(extension)) + extension
}

func truncateRelativePathSegments(value string) string {
	parts := strings.Split(value, "/")
	for index := range parts {
		parts[index] = TruncateFilenameSegment(parts[index], MaxFilenameSegmentBytes)
	}
	return strings.Join(parts, "/")
}

func truncateUTF8Bytes(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	for len(value) > limit {
		_, size := utf8.DecodeLastRuneInString(value)
		value = value[:len(value)-size]
	}
	return value
}

func metadataPathString(metadata map[string]interface{}, path string) string {
	if metadata == nil || path == "" {
		return ""
	}
	if direct, exists := metadata[path]; exists {
		return scalarString(direct)
	}
	var current interface{} = metadata
	for _, part := range strings.Split(path, ".") {
		object, ok := current.(map[string]interface{})
		if !ok {
			return ""
		}
		current, ok = object[part]
		if !ok {
			return ""
		}
	}
	return scalarString(current)
}

func metadataString(metadata map[string]interface{}, key string) string {
	if value, exists := metadata[key]; exists {
		return scalarString(value)
	}
	return ""
}

func scalarString(value interface{}) string {
	switch value := value.(type) {
	case string:
		return value
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(value), 'f', -1, 32)
	case int:
		return strconv.Itoa(value)
	case int64:
		return strconv.FormatInt(value, 10)
	case bool:
		return strconv.FormatBool(value)
	default:
		return ""
	}
}

func TruncateRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

func positiveIntegerString(value int) string {
	if value <= 0 {
		return ""
	}
	return strconv.Itoa(value)
}

func positiveInt64String(value int64) string {
	if value <= 0 {
		return ""
	}
	return strconv.FormatInt(value, 10)
}

func ResolveFilenameConflict(path, strategy string) (string, error) {
	return ResolveFilenameConflictWith(path, strategy, nil)
}

// ResolveFilenameConflictWith resolves a destination path while also treating
// paths reported by additionalConflict as occupied. This lets callers include
// in-flight downloads that have not created their destination files yet.
func ResolveFilenameConflictWith(path, strategy string, additionalConflict func(string) bool) (string, error) {
	conflicts := func(candidate string) bool {
		return shared.FileExist(candidate) || additionalConflict != nil && additionalConflict(candidate)
	}
	switch strategy {
	case "", "rename":
		if !conflicts(path) {
			return path, nil
		}
		extension := filepath.Ext(path)
		baseName := strings.TrimSuffix(path, extension)
		for count := 1; ; count++ {
			candidate := fmt.Sprintf("%s(%d)%s", baseName, count, extension)
			if !conflicts(candidate) {
				return candidate, nil
			}
		}
	case "overwrite":
		return path, nil
	case "skip":
		if additionalConflict != nil && additionalConflict(path) {
			return "", errors.New("destination already exists")
		}
		if _, err := os.Stat(path); err == nil {
			return "", errors.New("destination already exists")
		} else if !os.IsNotExist(err) {
			return "", err
		}
		return path, nil
	default:
		return "", fmt.Errorf("unsupported filename conflict strategy %q", strategy)
	}
}
