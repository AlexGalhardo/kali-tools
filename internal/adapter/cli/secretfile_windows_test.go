//go:build windows

package cli

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func grantRead(t *testing.T, path, sid string) {
	t.Helper()
	s, err := windows.StringToSid(sid)
	if err != nil {
		t.Fatal(err)
	}
	acl, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{{
		AccessPermissions: windows.GENERIC_READ, AccessMode: windows.GRANT_ACCESS,
		Trustee: windows.TRUSTEE{TrusteeForm: windows.TRUSTEE_IS_SID, TrusteeType: windows.TRUSTEE_IS_WELL_KNOWN_GROUP, TrusteeValue: windows.TrusteeValueFromSID(s)},
	}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.UNPROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil); err != nil {
		t.Fatal(err)
	}
}

func TestSecretFileOwnerWindowsACL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(path, []byte("synthetic-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := secretFileOwner(path, info); err != nil {
		t.Skipf("temp dir ACL is already permissive: %v", err)
	}
	for _, sid := range []string{"S-1-1-0", "S-1-5-32-545"} {
		grantRead(t, path, sid)
		if err := secretFileOwner(path, info); err == nil {
			t.Fatalf("file readable by %s accepted", sid)
		}
	}
}

func TestCheckSecretDACLNull(t *testing.T) {
	if checkSecretDACL(nil) == nil {
		t.Fatal("NULL DACL accepted")
	}
}
