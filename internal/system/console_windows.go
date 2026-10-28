package system

import (
	"os"

	"golang.org/x/sys/windows"
)

// PrepareCommandConsole attaches to the parent console while preserving inherited pipes.
func PrepareCommandConsole() {
	// Wails builds a GUI executable. Attach to the invoking console if present,
	// but preserve inherited pipes used by MCP hosts and shell redirection.
	attach := windows.NewLazySystemDLL("kernel32.dll").NewProc("AttachConsole")
	_, _, _ = attach.Call(uintptr(^uint32(0))) // ATTACH_PARENT_PROCESS
	for _, stream := range []struct {
		kind uint32
		file **os.File
		name string
	}{
		{windows.STD_INPUT_HANDLE, &os.Stdin, "stdin"},
		{windows.STD_OUTPUT_HANDLE, &os.Stdout, "stdout"},
		{windows.STD_ERROR_HANDLE, &os.Stderr, "stderr"},
	} {
		if handle := windows.Handle((*stream.file).Fd()); handle != 0 && handle != windows.InvalidHandle {
			continue
		}
		if handle, err := windows.GetStdHandle(stream.kind); err == nil && handle != 0 && handle != windows.InvalidHandle {
			*stream.file = os.NewFile(uintptr(handle), stream.name)
		}
	}
}
