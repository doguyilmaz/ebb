#include "bulk_darwin.h"

#include <errno.h>
#include <string.h>
#include <sys/attr.h>
#include <sys/resource.h>
#include <sys/vnode.h>
#include <unistd.h>

#define TAKE(dst, p) do { memcpy(&(dst), (p), sizeof(dst)); (p) += sizeof(dst); } while (0)

/* Reads one batch of directory entries. Attributes come packed in a fixed
   order: length, returned set, error, then common attributes in bit order,
   then directory attributes (directories only) or file attributes (everything
   else), then extended (fork) attributes. Even with FSOPT_PACK_INVAL_ATTRS the
   kernel skips the group that does not apply to the entry's type. Fields are
   only 4-byte aligned, hence memcpy. */
int ebb_bulk(int fd, char *buf, size_t cap, ebb_entry *out, int outcap,
             char *names, size_t namecap, size_t *nameused) {
	struct attrlist al;
	memset(&al, 0, sizeof al);
	al.bitmapcount = ATTR_BIT_MAP_COUNT;
	al.commonattr = ATTR_CMN_RETURNED_ATTRS | ATTR_CMN_NAME | ATTR_CMN_ERROR | ATTR_CMN_OBJTYPE |
	                ATTR_CMN_MODTIME | ATTR_CMN_FLAGS | ATTR_CMN_FILEID;
	al.dirattr = ATTR_DIR_MOUNTSTATUS;
	al.fileattr = ATTR_FILE_LINKCOUNT | ATTR_FILE_TOTALSIZE | ATTR_FILE_ALLOCSIZE;
	al.forkattr = ATTR_CMNEXT_CLONEID | ATTR_CMNEXT_EXT_FLAGS | ATTR_CMNEXT_CLONE_REFCNT;

	*nameused = 0;
	int n = getattrlistbulk(fd, &al, buf, cap, FSOPT_ATTR_CMN_EXTENDED | FSOPT_PACK_INVAL_ATTRS);
	if (n < 0) return -errno;
	if (n > outcap) n = outcap;

	char *entry = buf;
	for (int i = 0; i < n; i++) {
		ebb_entry *e = &out[i];
		memset(e, 0, sizeof *e);
		char *p = entry;
		uint32_t length;
		attribute_set_t ret;
		TAKE(length, p);
		TAKE(ret, p);

		uint32_t err = 0;
		TAKE(err, p);
		e->err = (ret.commonattr & ATTR_CMN_ERROR) ? err : 0;

		attrreference_t nref;
		char *nbase = p;
		TAKE(nref, p);
		uint32_t nlen = nref.attr_length > 0 ? nref.attr_length - 1 : 0;
		if (*nameused + nlen > namecap) return -ENOBUFS;
		memcpy(names + *nameused, nbase + nref.attr_dataoffset, nlen);
		e->name_off = (uint32_t)*nameused;
		e->name_len = (uint16_t)nlen;
		*nameused += nlen;

		fsobj_type_t type;
		struct timespec mtime;
		uint32_t flags;
		uint64_t fileid;
		TAKE(type, p);
		TAKE(mtime, p);
		TAKE(flags, p);
		TAKE(fileid, p);
		e->mtime = mtime.tv_sec;
		e->flags = flags;
		e->ino = fileid;
		switch (type) {
		case VDIR: e->kind = 0; break;
		case VREG: e->kind = 1; break;
		case VLNK: e->kind = 2; break;
		default: e->kind = 3;
		}

		if (type == VDIR) {
			uint32_t mstatus;
			TAKE(mstatus, p);
			if ((ret.dirattr & ATTR_DIR_MOUNTSTATUS) && (mstatus & DIR_MNTSTATUS_MNTPOINT)) e->mount = 1;
		} else {
			uint32_t nlink;
			off_t total, alloc;
			TAKE(nlink, p);
			TAKE(total, p);
			TAKE(alloc, p);
			if (ret.fileattr & ATTR_FILE_LINKCOUNT) e->nlink = nlink;
			if (ret.fileattr & ATTR_FILE_TOTALSIZE) e->logical = (uint64_t)total;
			if (ret.fileattr & ATTR_FILE_ALLOCSIZE) e->alloc = (uint64_t)alloc;
		}

		uint64_t cid, ext;
		uint32_t refs;
		TAKE(cid, p);
		TAKE(ext, p);
		TAKE(refs, p);
		if (ret.forkattr & ATTR_CMNEXT_EXT_FLAGS) e->ext = (uint32_t)ext;
		if ((ret.forkattr & ATTR_CMNEXT_CLONEID) && (ret.forkattr & ATTR_CMNEXT_CLONE_REFCNT)) {
			e->clone_id = cid;
			e->clone_refs = refs;
		}

		entry += length;
	}
	return n;
}

int ebb_set_iopolicy(int throttle) {
	return setiopolicy_np(IOPOL_TYPE_DISK, IOPOL_SCOPE_PROCESS, throttle ? IOPOL_THROTTLE : IOPOL_UTILITY);
}
