package naming

import (
	"errors"
	"regexp"
	"strings"
)

const maxFilenameRegexBytes = 64 * 1024

var (
	errFilenameRegexSize    = errors.New("filename regex_replace input or result exceeds 64 KiB")
	errFilenameRegexMatches = errors.New("filename regex_replace has too many capture matches")
)

func replaceFilenameRegex(pattern *regexp.Regexp, input, replacement string) (string, error) {
	if len(input) > maxFilenameRegexBytes {
		return "", errFilenameRegexSize
	}

	// Deletion can only shrink the input. Let regexp stream this common case.
	if replacement == "" {
		return pattern.ReplaceAllString(input, ""), nil
	}

	// Find all matches on the original input to preserve anchors, boundaries
	// and Go's handling of adjacent empty matches. Bound the retained capture
	// offsets as well, even for patterns with many groups and tiny matches.
	const maxCaptureOffsets = 1 << 20
	maxMatches := maxCaptureOffsets / (2 * (1 + pattern.NumSubexp()))
	matches := pattern.FindAllStringSubmatchIndex(input, maxMatches+1)
	if len(matches) > maxMatches {
		return "", errFilenameRegexMatches
	}
	if len(matches) == 0 {
		return input, nil
	}

	// Expand at most one capture per piece. Even one match can otherwise
	// allocate the entire input many times before we can check the result.
	pieces := filenameReplacementPieces(replacement)
	var output strings.Builder
	var scratch []byte
	end := 0
	for _, match := range matches {
		if match[0]-end > maxFilenameRegexBytes-output.Len() {
			return "", errFilenameRegexSize
		}
		output.WriteString(input[end:match[0]])
		for _, piece := range pieces {
			scratch = pattern.ExpandString(scratch[:0], piece, input, match)
			if len(scratch) > maxFilenameRegexBytes-output.Len() {
				return "", errFilenameRegexSize
			}
			output.Write(scratch)
		}
		end = match[1]
	}
	if len(input)-end > maxFilenameRegexBytes-output.Len() {
		return "", errFilenameRegexSize
	}
	output.WriteString(input[end:])
	return output.String(), nil
}

// Split before each dollar token, keeping $$ together. ExpandString still
// interprets all capture names, malformed references and dollar escapes.
func filenameReplacementPieces(value string) []string {
	var pieces []string
	for len(value) > 0 {
		start := 0
		if strings.HasPrefix(value, "$$") {
			start = 2
		} else if value[0] == '$' {
			start = 1
		}
		next := strings.IndexByte(value[start:], '$')
		if next < 0 {
			return append(pieces, value)
		}
		end := start + next
		pieces = append(pieces, value[:end])
		value = value[end:]
	}
	return pieces
}
