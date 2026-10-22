package app

import (
	"context"
	"fmt"

	"github.com/AlexGalhardo/kali-tools/internal/tlsaudit"
)

func (a *Application) AuditTLS(ctx context.Context, r tlsaudit.Request) (report tlsaudit.Report, err error) {
	if err := tlsaudit.Validate(r); err != nil {
		return report, err
	}
	err = a.runOperation(ctx, "tls_audit", func(ctx context.Context) (string, error) {
		var auditErr error
		report, auditErr = tlsaudit.Audit(ctx, r)
		errors, warnings := 0, 0
		for _, f := range report.Findings {
			switch f.Severity {
			case "fatal", "error":
				errors++
			case "warn":
				warnings++
			}
		}
		return fmt.Sprintf("%d certificates, %d errors, %d warnings", len(report.Chain), errors, warnings), auditErr
	})
	return report, err
}
