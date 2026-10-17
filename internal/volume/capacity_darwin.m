#import <Foundation/Foundation.h>
#include "capacity_darwin.h"

/* Available-for-important-usage includes space macOS will purge on demand
   (caches, local snapshots); the plain available capacity does not. */
int ebb_capacity(const char *path, int64_t *available, int64_t *important) {
	@autoreleasepool {
		NSURL *url = [NSURL fileURLWithPath:[NSString stringWithUTF8String:path]];
		NSDictionary *d = [url resourceValuesForKeys:@[ NSURLVolumeAvailableCapacityKey, NSURLVolumeAvailableCapacityForImportantUsageKey ] error:nil];
		if (d == nil) return -1;
		*available = [d[NSURLVolumeAvailableCapacityKey] longLongValue];
		*important = [d[NSURLVolumeAvailableCapacityForImportantUsageKey] longLongValue];
		return 0;
	}
}
