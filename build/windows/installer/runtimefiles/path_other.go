//go:build !windows

package main

import "io/fs"

func payloadEntryAllowed(info fs.FileInfo) bool {
	return info.IsDir() || info.Mode().IsRegular()
}
