//go:build darwin && cgo

package system

/*
#cgo LDFLAGS: -framework CoreFoundation
#include <CoreFoundation/CoreFoundation.h>

static void readPreferredLanguage(char *buffer, CFIndex capacity) {
    buffer[0] = '\0';
    CFArrayRef languages = CFLocaleCopyPreferredLanguages();
    if (languages == NULL) return;
    if (CFArrayGetCount(languages) > 0) {
        CFTypeRef first = CFArrayGetValueAtIndex(languages, 0);
        if (CFGetTypeID(first) == CFStringGetTypeID()) {
            if (!CFStringGetCString((CFStringRef)first, buffer, capacity, kCFStringEncodingUTF8)) {
                buffer[0] = '\0';
            }
        }
    }
    CFRelease(languages);
}
*/
import "C"

func preferredLanguage() string {
	var buffer [128]C.char
	C.readPreferredLanguage(&buffer[0], C.CFIndex(len(buffer)))
	return C.GoString(&buffer[0])
}
