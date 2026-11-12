#import <Foundation/Foundation.h>
#include <stdlib.h>
#include <string.h>
#include "norm_darwin.h"

/* Returns s with its letters composed (Unicode NFC) in memory the caller
   frees, or NULL when s is not UTF-8. */
char *ebb_composed(const char *s) {
	@autoreleasepool {
		NSString *str = [NSString stringWithUTF8String:s];
		if (str == nil) return NULL;
		const char *out = [str precomposedStringWithCanonicalMapping].UTF8String;
		return out ? strdup(out) : NULL;
	}
}
