#include <stddef.h>
#include <stdint.h>

typedef struct ebb_watch ebb_watch;

uint64_t ebb_fse_current(void);
/* Writes the FSEvents database UUID of the volume holding path; returns 0 on success. */
int ebb_fse_uuid(const char *path, char *out, int cap);
/* since: an event ID, or 0 for from now. handle is passed back to ebbFSEvents. */
ebb_watch *ebb_watch_start(const char *root, uint64_t since, double latency, uintptr_t handle);
/* Delivers every event that happened so far before it returns. */
void ebb_watch_flush(ebb_watch *w);
/* Stops the stream; no callback runs after it returns. */
void ebb_watch_stop(ebb_watch *w);
