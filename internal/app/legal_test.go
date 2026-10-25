package app_test

import (
	"errors"
	"testing"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

func TestLegalNoticeGatesActiveScansUntilAccepted(t *testing.T) {
	deps := fakeDeps()
	deps.Settings = &fakeSettings{}
	a := app.New(deps)
	defer func() { _ = a.Close() }()
	ctx := t.Context()
	if n, err := a.LegalNotice(ctx); err != nil || n.Accepted || n.Version != app.LegalNoticeVersion {
		t.Fatalf("fresh notice = %+v, %v", n, err)
	}
	var e *domain.Error
	_, _, err := a.Discover(ctx, "127.0.0.1/32", []int{1}, time.Second)
	if !errors.As(err, &e) || e.Code != domain.CodeForbidden {
		t.Fatalf("Discover before ack = %v", err)
	}
	_, err = a.InspectServices(ctx, "127.0.0.1/32", []int{1})
	if !errors.As(err, &e) || e.Code != domain.CodeForbidden {
		t.Fatalf("InspectServices before ack = %v", err)
	}
	if err := a.AcceptLegalNotice(ctx); err != nil {
		t.Fatal(err)
	}
	if n, _ := a.LegalNotice(ctx); !n.Accepted {
		t.Fatal("not accepted after ack")
	}
}

func TestSaveSettingsKeepsLegalAck(t *testing.T) {
	a := app.New(fakeDeps())
	defer func() { _ = a.Close() }()
	ctx := t.Context()
	if err := a.AcceptLegalNotice(ctx); err != nil {
		t.Fatal(err)
	}
	if err := a.SaveSettings(ctx, app.Settings{Language: "pt-BR", RetentionDays: 7}); err != nil {
		t.Fatal(err)
	}
	if n, _ := a.LegalNotice(ctx); !n.Accepted {
		t.Fatal("SaveSettings cleared the acknowledgement")
	}
}
