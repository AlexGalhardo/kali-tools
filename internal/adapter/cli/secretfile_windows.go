//go:build windows

package cli

import (
	"os"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

const readMask = windows.FILE_READ_DATA | windows.GENERIC_READ | windows.GENERIC_ALL

// secretFileOwner requires the file owner to be the current token user and
// rejects a DACL that lets Everyone or BUILTIN\Users read the file.
func secretFileOwner(path string, _ os.FileInfo) error {
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return domain.Invalid("secret file permissions cannot be checked")
	}
	owner, _, err := sd.Owner()
	if err != nil {
		return domain.Invalid("secret file permissions cannot be checked")
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return domain.Invalid("secret file permissions cannot be checked")
	}
	if !owner.Equals(user.User.Sid) {
		return domain.Invalid("secret file must be owned by the user")
	}
	acl, _, err := sd.DACL()
	if err != nil {
		return domain.Invalid("secret file permissions cannot be checked")
	}
	return checkSecretDACL(acl)
}

func checkSecretDACL(acl *windows.ACL) error {
	weak := domain.Invalid("secret file must not be readable by Everyone or Users")
	if acl == nil {
		return weak // a NULL DACL grants everyone full access
	}
	everyone, err := windows.StringToSid("S-1-1-0")
	if err != nil {
		return weak
	}
	users, err := windows.StringToSid("S-1-5-32-545")
	if err != nil {
		return weak
	}
	for i := uint32(0); i < uint32(acl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(acl, i, &ace); err != nil {
			return weak
		}
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE || ace.Mask&readMask == 0 {
			continue
		}
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		if sid.Equals(everyone) || sid.Equals(users) {
			return weak
		}
	}
	return nil
}
