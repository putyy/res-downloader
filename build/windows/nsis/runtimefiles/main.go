// Command runtimefiles generates a compile-time NSIS allowlist from the bundled
// WebView2 payload. It runs only on the build machine, never during uninstall.
package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: runtimefiles SOURCE OUTPUT")
		os.Exit(1)
	}
	if err := generate(os.Args[1], os.Args[2]); err != nil {
		fmt.Fprintln(os.Stderr, "generate runtime file list:", err)
		os.Exit(1)
	}
}

func windowsLength(value string) int {
	return len(utf16.Encode([]rune(value)))
}

func foldedPath(value string) string {
	return strings.Map(func(char rune) rune {
		key := char
		for folded := unicode.SimpleFold(char); folded != char; folded = unicode.SimpleFold(folded) {
			key = min(key, folded)
		}
		return key
	}, value)
}

func generate(source, output string) error {
	source, err := filepath.Abs(source)
	if err != nil {
		return err
	}
	root, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if !root.IsDir() || !payloadEntryAllowed(root) {
		return fmt.Errorf("runtime payload root must be a regular directory: %s", source)
	}
	var files, directories []string
	seen := make(map[string]bool)
	longestPath := 0
	hasExecutable := false
	err = filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == source {
			return nil
		}
		info, err := entry.Info() // Lstat semantics: do not follow directory links.
		if err != nil {
			return err
		}
		if !payloadEntryAllowed(info) {
			return fmt.Errorf("runtime payload contains a link or unsupported file: %s", path)
		}
		name := entry.Name()
		if !utf8.ValidString(name) || name == "." || name == ".." ||
			strings.HasSuffix(name, ".") || strings.HasSuffix(name, " ") ||
			strings.ContainsAny(name, "$\"<>:|?*\\\r\n") ||
			strings.ContainsFunc(name, func(char rune) bool { return char < 32 }) {
			return fmt.Errorf("unsafe runtime filename: %s", path)
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		relative = strings.ReplaceAll(filepath.ToSlash(relative), "/", "\\")
		length := windowsLength(relative)
		key := foldedPath(relative)
		if length > 700 || seen[key] {
			return fmt.Errorf("ambiguous or overlong runtime filename: %s", relative)
		}
		seen[key] = true
		longestPath = max(longestPath, length)
		if info.IsDir() {
			directories = append(directories, relative)
		} else {
			files = append(files, relative)
			hasExecutable = hasExecutable || relative == "msedgewebview2.exe"
		}
		return nil
	})
	if err != nil {
		return err
	}
	if !hasExecutable {
		return fmt.Errorf("runtime payload must contain msedgewebview2.exe")
	}

	var script strings.Builder
	script.WriteString("; Generated from the build payload. Do not edit.\n")
	script.WriteString("!macro resd.RecordRuntime\n    SetRegView 64\n    ClearErrors\n")
	for _, name := range files {
		fmt.Fprintf(&script, "    WriteRegStr HKLM \"${RESD_RUNTIME_FILES_KEY}\" \"%s\" \"F\"\n", name)
	}
	for _, name := range directories {
		fmt.Fprintf(&script, "    WriteRegStr HKLM \"${RESD_RUNTIME_FILES_KEY}\" \"%s\" \"D\"\n", name)
	}
	script.WriteString(`    ${If} ${Errors}
        MessageBox MB_ICONSTOP|MB_OK "Could not record WebView2 Runtime files. No runtime files have been written. Retry the installation." /SD IDOK
        SetErrorLevel 2
        Abort
    ${EndIf}
!macroend
`)
	fmt.Fprintf(&script, "!define RESD_RUNTIME_MAX_PATH_LENGTH %d\n", len("WebView2Runtime\\")+longestPath)
	script.WriteString(`!macro resd.RuntimePreflight PREFIX
    StrLen $0 "$INSTDIR"
    IntOp $0 $0 + ${RESD_RUNTIME_MAX_PATH_LENGTH}
    IntOp $0 $0 + 1
    ${If} $0 >= ${NSIS_MAX_STRLEN}
        MessageBox MB_ICONSTOP|MB_OK "The installation path is too long. Choose a shorter application folder." /SD IDOK
        SetErrorLevel 2
        Abort
    ${EndIf}
`)
	paths := append([]string{""}, directories...)
	paths = append(paths, files...)
	for _, name := range paths {
		path := "$INSTDIR\\WebView2Runtime"
		if name != "" {
			path += "\\" + name
		}
		fmt.Fprintf(&script, "    Push \"%s\"\n    Call ${PREFIX}resd.RequirePlainPath\n", path)
	}
	script.WriteString("!macroend\n!macro resd.RemoveRuntime\n")
	for _, name := range files {
		fmt.Fprintf(&script, "    !insertmacro resd.DeleteFile \"$INSTDIR\\WebView2Runtime\\%s\"\n", name)
	}
	sort.Slice(directories, func(i, j int) bool {
		left, right := strings.Count(directories[i], "\\"), strings.Count(directories[j], "\\")
		if left != right {
			return left > right
		}
		return directories[i] > directories[j]
	})
	for _, name := range directories {
		fmt.Fprintf(&script, "    !insertmacro resd.RemoveEmptyDirectory \"$INSTDIR\\WebView2Runtime\\%s\"\n", name)
	}
	script.WriteString("    !insertmacro resd.RemoveEmptyDirectory \"$INSTDIR\\WebView2Runtime\"\n!macroend\n")
	return os.WriteFile(output, []byte(script.String()), 0600)
}
