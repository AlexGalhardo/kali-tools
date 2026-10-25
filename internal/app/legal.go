package app

import (
	"context"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

// LegalNoticeVersion is the revision of the authorized-use notice. Bump it when the
// text changes materially so previous acknowledgements stop counting.
const LegalNoticeVersion = "1"

// LegalNotice reports which notice revision applies and whether it was acknowledged.
type LegalNotice struct {
	Version  string `json:"version"`
	Accepted bool   `json:"accepted"`
}

func (a *Application) LegalNotice(ctx context.Context) (LegalNotice, error) {
	s, err := a.deps.Settings.Settings(ctx)
	return LegalNotice{Version: LegalNoticeVersion, Accepted: s.LegalNoticeAck == LegalNoticeVersion}, err
}

func (a *Application) AcceptLegalNotice(ctx context.Context) error {
	s, err := a.deps.Settings.Settings(ctx)
	if err != nil {
		return err
	}
	s.LegalNoticeAck = LegalNoticeVersion
	return a.deps.Settings.SaveSettings(ctx, s)
}

// requireLegalNotice gates active scans only; passive features never call it.
func (a *Application) requireLegalNotice(ctx context.Context) error {
	n, err := a.LegalNotice(ctx)
	if err != nil {
		return err
	}
	if !n.Accepted {
		return &domain.Error{Code: domain.CodeForbidden, Message: "active scans need the authorized-use notice to be accepted first; run `kali-tools legal --accept` or acknowledge it in the app"}
	}
	return nil
}
