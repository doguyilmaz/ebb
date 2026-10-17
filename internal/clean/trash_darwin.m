#import <Foundation/Foundation.h>
#include <string.h>
#include "trash_darwin.h"

/* Moves path to the Trash the way Finder does (Put Back works) and writes the
   item's new path into out. Returns 0 on success. */
int ebb_trash(const char *path, char *out, int outcap) {
	@autoreleasepool {
		NSURL *url = [NSURL fileURLWithPath:[NSString stringWithUTF8String:path]];
		NSURL *result = nil;
		NSError *err = nil;
		if (![[NSFileManager defaultManager] trashItemAtURL:url resultingItemURL:&result error:&err]) {
			return err ? (int)err.code : -1;
		}
		const char *p = result.path.fileSystemRepresentation;
		if (strlen(p) + 1 > (size_t)outcap) return -2;
		strcpy(out, p);
		return 0;
	}
}
