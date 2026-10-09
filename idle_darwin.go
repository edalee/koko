//go:build darwin

package main

/*
#cgo LDFLAGS: -framework IOKit -framework CoreFoundation
#include <IOKit/IOKitLib.h>
#include <CoreFoundation/CoreFoundation.h>

// hidIdleNanos returns IOHIDSystem's HIDIdleTime: nanoseconds since the last
// key press, click or mouse move, in any app. It returns -1 if it cannot be
// read. It needs no permission, unlike the CGEventSource calls.
static long long hidIdleNanos(void) {
	io_service_t service = IOServiceGetMatchingService(MACH_PORT_NULL, IOServiceMatching("IOHIDSystem"));
	if (service == IO_OBJECT_NULL) return -1;
	CFTypeRef prop = IORegistryEntryCreateCFProperty(service, CFSTR("HIDIdleTime"), kCFAllocatorDefault, 0);
	IOObjectRelease(service);
	if (prop == NULL) return -1;
	long long nanos = -1;
	if (CFGetTypeID(prop) == CFNumberGetTypeID()) {
		if (!CFNumberGetValue((CFNumberRef)prop, kCFNumberSInt64Type, &nanos)) nanos = -1;
	}
	CFRelease(prop);
	return nanos;
}
*/
import "C"

// idleSeconds returns the seconds since the user's last input in any app, or
// -1 if macOS does not say.
func idleSeconds() float64 {
	n := C.hidIdleNanos()
	if n < 0 {
		return -1
	}
	return float64(n) / 1e9
}
