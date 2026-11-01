package main

import (
	"io/fs"
	"syscall"
)

func payloadEntryAllowed(info fs.FileInfo) bool {
	attributes, ok := info.Sys().(*syscall.Win32FileAttributeData)
	return ok && attributes.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT == 0 &&
		(info.IsDir() || info.Mode().IsRegular())
}
