#include "fsevents_darwin.h"

#include <CoreServices/CoreServices.h>
#include <dispatch/dispatch.h>
#include <stdlib.h>
#include <sys/stat.h>

extern void ebbFSEvents(uintptr_t handle, size_t n, char **paths, uint32_t *flags, uint64_t *ids);

struct ebb_watch {
	FSEventStreamRef stream;
	dispatch_queue_t queue;
	uintptr_t handle;
};

uint64_t ebb_fse_current(void) { return FSEventsGetCurrentEventId(); }

int ebb_fse_uuid(const char *path, char *out, int cap) {
	struct stat st;
	if (stat(path, &st) != 0) return -1;
	CFUUIDRef uuid = FSEventsCopyUUIDForDevice(st.st_dev);
	if (!uuid) return -1;
	CFStringRef s = CFUUIDCreateString(NULL, uuid);
	Boolean ok = CFStringGetCString(s, out, cap, kCFStringEncodingUTF8);
	CFRelease(s);
	CFRelease(uuid);
	return ok ? 0 : -1;
}

static void callback(ConstFSEventStreamRef stream, void *info, size_t n, void *paths, const FSEventStreamEventFlags flags[],
                     const FSEventStreamEventId ids[]) {
	ebb_watch *w = info;
	uint32_t *f = malloc(n * sizeof *f);
	uint64_t *id = malloc(n * sizeof *id);
	if (!f || !id) {
		free(f);
		free(id);
		return;
	}
	for (size_t i = 0; i < n; i++) {
		f[i] = flags[i];
		id[i] = ids[i];
	}
	ebbFSEvents(w->handle, n, paths, f, id);
	free(f);
	free(id);
}

ebb_watch *ebb_watch_start(const char *root, uint64_t since, double latency, uintptr_t handle) {
	ebb_watch *w = calloc(1, sizeof *w);
	if (!w) return NULL;
	w->handle = handle;
	CFStringRef path = CFStringCreateWithCString(NULL, root, kCFStringEncodingUTF8);
	CFArrayRef paths = CFArrayCreate(NULL, (const void **)&path, 1, &kCFTypeArrayCallBacks);
	FSEventStreamContext ctx = {0, w, NULL, NULL, NULL};
	w->stream = FSEventStreamCreate(NULL, callback, &ctx, paths, since ? since : kFSEventStreamEventIdSinceNow, latency,
	                                kFSEventStreamCreateFlagNoDefer);
	CFRelease(paths);
	CFRelease(path);
	if (!w->stream) {
		free(w);
		return NULL;
	}
	w->queue = dispatch_queue_create("ebb.fsevents", DISPATCH_QUEUE_SERIAL);
	FSEventStreamSetDispatchQueue(w->stream, w->queue);
	if (!FSEventStreamStart(w->stream)) {
		FSEventStreamInvalidate(w->stream);
		FSEventStreamRelease(w->stream);
		dispatch_release(w->queue);
		free(w);
		return NULL;
	}
	return w;
}

void ebb_watch_stop(ebb_watch *w) {
	/* On the stream's own queue, so a callback already running finishes first. */
	dispatch_sync(w->queue, ^{
		FSEventStreamStop(w->stream);
		FSEventStreamInvalidate(w->stream);
	});
	FSEventStreamRelease(w->stream);
	dispatch_release(w->queue);
	free(w);
}
