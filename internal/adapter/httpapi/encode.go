package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/adapter/contract"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

const maxJSONBody = 64 << 10
const bodyReadTimeout = 30 * time.Second

type transportError struct {
	status int
	cause  error
}

func (e transportError) Error() string { return e.cause.Error() }
func (e transportError) Unwrap() error { return e.cause }

func requestError(status int, code, message string) error {
	return transportError{status: status, cause: &domain.Error{Code: code, Message: message}}
}

func statusFor(code string) int {
	switch code {
	case domain.CodeInvalidInput:
		return http.StatusBadRequest
	case domain.CodeUnauthorized:
		return http.StatusUnauthorized
	case domain.CodeForbidden, domain.CodePermissionRequired, domain.CodeLocationConsentRequired:
		return http.StatusForbidden
	case domain.CodeNotFound:
		return http.StatusNotFound
	case domain.CodeConflict:
		return http.StatusConflict
	case domain.CodeDependencyMissing:
		return http.StatusFailedDependency
	case domain.CodeCancelled:
		return 499
	case domain.CodeUnsupported:
		return http.StatusNotImplemented
	case domain.CodeNoConnection, domain.CodeCertificateInvalid:
		return http.StatusBadGateway
	case domain.CodeTimeout:
		return http.StatusGatewayTimeout
	default:
		return http.StatusInternalServerError
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if value != nil {
		_ = json.NewEncoder(w).Encode(value)
	}
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	mapped := contract.FromError(err)
	status := statusFor(mapped.Code)
	var failure transportError
	if errors.As(err, &failure) {
		status = failure.status
	}
	writeJSON(w, status, struct {
		Error contract.Error `json:"error"`
	}{Error: mapped})
}

func bodyError(err error) error {
	var oversized *http.MaxBytesError
	if errors.As(err, &oversized) {
		return requestError(http.StatusRequestEntityTooLarge, domain.CodeInvalidInput, "request body too large")
	}
	return domain.Invalid("invalid request body")
}

func decodeJSON(w http.ResponseWriter, r *http.Request, value any, limit int64) error {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || !strings.EqualFold(media, "application/json") {
		return requestError(http.StatusUnsupportedMediaType, domain.CodeInvalidInput, "expected application/json")
	}
	rc := http.NewResponseController(w)
	if err := rc.SetReadDeadline(time.Now().Add(bodyReadTimeout)); err != nil && !errors.Is(err, http.ErrNotSupported) {
		return err
	}
	defer func() { _ = rc.SetReadDeadline(time.Time{}) }()
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return bodyError(err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return bodyError(err)
		}
		return domain.Invalid("unexpected trailing data")
	}
	return nil
}

func decodeEmpty(w http.ResponseWriter, r *http.Request) error {
	rc := http.NewResponseController(w)
	if err := rc.SetReadDeadline(time.Now().Add(bodyReadTimeout)); err != nil && !errors.Is(err, http.ErrNotSupported) {
		return err
	}
	defer func() { _ = rc.SetReadDeadline(time.Time{}) }()
	r.Body = http.MaxBytesReader(w, r.Body, 0)
	if _, err := io.Copy(io.Discard, r.Body); err != nil {
		return bodyError(err)
	}
	return nil
}

func emptyRequest(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := decodeEmpty(w, r); err != nil {
			writeError(w, r, err)
			return
		}
		next.ServeHTTP(w, r)
	}
}
