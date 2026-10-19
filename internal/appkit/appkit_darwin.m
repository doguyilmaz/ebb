#import <AppKit/AppKit.h>
#import <UserNotifications/UserNotifications.h>
#include "appkit_darwin.h"
#include "_cgo_export.h"

@interface EbbApp : NSObject <NSApplicationDelegate, UNUserNotificationCenterDelegate>
@end

@implementation EbbApp
// Opening ebb.app again while the service runs comes here instead of starting a second copy.
- (BOOL)applicationShouldHandleReopen:(NSApplication *)app hasVisibleWindows:(BOOL)visible {
	ebbOpenWindow();
	return NO;
}

- (void)userNotificationCenter:(UNUserNotificationCenter *)center
    didReceiveNotificationResponse:(UNNotificationResponse *)response
             withCompletionHandler:(void (^)(void))done {
	if ([response.actionIdentifier isEqualToString:UNNotificationDefaultActionIdentifier]) ebbOpenWindow();
	done();
}

- (void)userNotificationCenter:(UNUserNotificationCenter *)center
       willPresentNotification:(UNNotification *)notification
         withCompletionHandler:(void (^)(UNNotificationPresentationOptions))done {
	done(UNNotificationPresentationOptionBanner | UNNotificationPresentationOptionList);
}
@end

// Outside a bundle the notification center aborts the process, and no @try catches it.
int ebb_bundled(void) {
	return NSBundle.mainBundle.bundleIdentifier != nil;
}

void ebb_run(void) {
	static EbbApp *delegate;
	delegate = [EbbApp new];
	[NSApplication sharedApplication];
	[NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
	NSApp.delegate = delegate;
	UNUserNotificationCenter *center = UNUserNotificationCenter.currentNotificationCenter;
	center.delegate = delegate;
	[center requestAuthorizationWithOptions:(UNAuthorizationOptionAlert | UNAuthorizationOptionSound)
	                      completionHandler:^(BOOL granted, NSError *error){
	                      }];
	[NSApp run];
}

void ebb_notify(const char *title, const char *body) {
	UNMutableNotificationContent *content = [UNMutableNotificationContent new];
	content.title = [NSString stringWithUTF8String:title];
	content.body = [NSString stringWithUTF8String:body];
	content.sound = UNNotificationSound.defaultSound;
	UNNotificationRequest *request = [UNNotificationRequest requestWithIdentifier:NSUUID.UUID.UUIDString content:content trigger:nil];
	[UNUserNotificationCenter.currentNotificationCenter addNotificationRequest:request withCompletionHandler:nil];
}
