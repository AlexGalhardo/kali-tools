package domain

import (
	"fmt"
	"slices"
)

const SchemaVersion = 1
const Version = "0.6.0-dev.2"

// LocationSettingsURI is the only URI opened for Wi-Fi consent.
const LocationSettingsURI = "ms-settings:privacy-location"

const NpcapDownloadURL = "https://npcap.com/#download"

// Capability states (Capability.State).
const (
	CapabilityAvailable               = "available"
	CapabilityDependencyMissing       = "dependency_missing"
	CapabilityPermissionRequired      = "permission_required"
	CapabilityLocationConsentRequired = "location_consent_required"
	CapabilityUnsupported             = "unsupported"
	CapabilityFailed                  = "failed"
)

// Stable error codes (Error.Code). Adding one requires both i18n catalogs.
const (
	CodeInvalidInput            = "invalid_input"
	CodeUnsupported             = "unsupported"
	CodePermissionRequired      = "permission_required"
	CodeDependencyMissing       = "dependency_missing"
	CodeLocationConsentRequired = "location_consent_required"
	CodeNotFound                = "not_found"
	CodeTimeout                 = "timeout"
	CodeCancelled               = "cancelled"
	CodeFailed                  = "failed"
	CodeCertificateInvalid      = "certificate_invalid"
	CodeNoConnection            = "no_connection"
	CodeConflict                = "conflict"
	CodeUnauthorized            = "unauthorized"
	CodeForbidden               = "forbidden"
)

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }
func Invalid(format string, args ...any) error {
	return &Error{CodeInvalidInput, fmt.Sprintf(format, args...)}
}
func Unsupported(message string) error { return &Error{CodeUnsupported, message} }

type Capability struct {
	ID       string `json:"id"`
	State    string `json:"state"`
	Detail   string `json:"detail"`
	Provider string `json:"provider"`
}

func CapabilityStates() []string {
	return []string{CapabilityAvailable, CapabilityDependencyMissing, CapabilityPermissionRequired, CapabilityLocationConsentRequired, CapabilityUnsupported, CapabilityFailed}
}
func ErrorCodes() []string {
	return []string{CodeInvalidInput, CodeUnsupported, CodePermissionRequired, CodeDependencyMissing, CodeLocationConsentRequired, CodeNotFound, CodeTimeout, CodeCancelled, CodeFailed, CodeCertificateInvalid, CodeNoConnection, CodeConflict, CodeUnauthorized, CodeForbidden}
}
func IsCapabilityState(s string) bool { return slices.Contains(CapabilityStates(), s) }
func IsErrorCode(code string) bool    { return slices.Contains(ErrorCodes(), code) }
