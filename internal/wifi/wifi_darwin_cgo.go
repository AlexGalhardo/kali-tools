//go:build darwin && cgo

package wifi

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework CoreWLAN -framework Foundation -framework Security
#import <CoreWLAN/CoreWLAN.h>
#import <Security/Security.h>
#import <stdlib.h>

static int ktCurrent(char **ssid, char **bssid, int *channel, int *rssi) {
	@autoreleasepool {
		*ssid = NULL;
		*bssid = NULL;
		*channel = 0;
		*rssi = 0;
		CWInterface *iface = [[CWWiFiClient sharedWiFiClient] interface];
		if (iface == nil || [iface interfaceName] == nil) {
			return 1;
		}
		NSString *name = [iface ssid];
		if (name == nil || [name length] == 0 || [name UTF8String] == NULL) {
			return 2;
		}
		*ssid = strdup([name UTF8String]);
		NSString *bssidValue = [iface bssid];
		if (bssidValue != nil && [bssidValue UTF8String] != NULL) {
			*bssid = strdup([bssidValue UTF8String]);
		}
		CWChannel *wlanChannel = [iface wlanChannel];
		if (wlanChannel != nil) {
			*channel = (int)[wlanChannel channelNumber];
		}
		*rssi = (int)[iface rssiValue];
		return 0;
	}
}

static int ktReveal(const char *account, char **secret, int *status) {
	@autoreleasepool {
		*secret = NULL;
		*status = 0;
		NSString *name = account == NULL ? @"" : [NSString stringWithUTF8String:account];
		NSDictionary *query = @{
			(id)kSecClass : (id)kSecClassGenericPassword,
			(id)kSecAttrService : @"AirPort",
			(id)kSecAttrAccount : name,
			(id)kSecReturnData : @YES,
			(id)kSecMatchLimit : (id)kSecMatchLimitOne,
			(id)kSecUseAuthenticationUI : (id)kSecUseAuthenticationUIFail
		};
		CFTypeRef item = NULL;
		OSStatus result = SecItemCopyMatching((__bridge CFDictionaryRef)query, &item);
		*status = (int)result;
		if (result != errSecSuccess || item == NULL) {
			return -1;
		}
		NSData *data = (__bridge_transfer NSData *)item;
		if ([data length] == 0) {
			*status = -25300;
			return -1;
		}
		if ([data length] > 256) {
			*status = 3;
			return -1;
		}
		*secret = strndup((const char *)[data bytes], [data length]);
		return 0;
	}
}
*/
import "C"
import "unsafe"

func Current() (Network, error) {
	var ssid, bssid *C.char
	var channel, rssi C.int
	code := int(C.ktCurrent(&ssid, &bssid, &channel, &rssi))
	return networkFromCoreWLAN(code, takeCString(ssid), takeCString(bssid), int(channel), int(rssi))
}

func Reveal() (Secret, error) {
	network, err := Current()
	if err != nil {
		return Secret{}, err
	}
	account := C.CString(network.SSID)
	defer C.free(unsafe.Pointer(account))
	var secret *C.char
	var status C.int
	C.ktReveal(account, &secret, &status)
	return secretFromKeychain(int(status), takeCString(secret))
}

func takeCString(value *C.char) string {
	if value == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(value))
	return C.GoString(value)
}
