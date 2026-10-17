#include <stddef.h>
#include <stdint.h>

typedef struct {
	uint64_t ino, alloc, logical, clone_id;
	int64_t mtime;
	uint32_t nlink, flags, ext, name_off, err, clone_refs;
	uint16_t name_len;
	uint8_t kind, mount;
} ebb_entry;

int ebb_bulk(int fd, char *attrbuf, size_t attrcap, ebb_entry *out, int outcap,
             char *names, size_t namecap, size_t *nameused, int want_clone);
int ebb_set_iopolicy(int throttle);
/* openat; returns the descriptor or -errno. */
int ebb_openat(int dirfd, const char *name, int flags);
