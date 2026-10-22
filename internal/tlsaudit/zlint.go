package tlsaudit

import (
	"fmt"
	"io"
	"runtime/debug"
	"slices"
	"sync"

	"github.com/sirupsen/logrus"
	zx509 "github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3"
	"github.com/zmap/zlint/v3/lint"
)

var silenceLint = sync.OnceFunc(func() {
	// Process-global logrus output is silenced to keep certificate data out of stderr (B08).
	logrus.SetOutput(io.Discard)
})

func zlintFindings(der []byte) ([]Finding, string, error) {
	silenceLint()
	cert, err := zx509.ParseCertificate(der)
	if err != nil {
		return nil, "", fmt.Errorf("parsing lint certificate: %w", err)
	}
	results := zlint.LintCertificate(cert)
	names := make([]string, 0, len(results.Results))
	for name := range results.Results {
		names = append(names, name)
	}
	slices.Sort(names)
	report := Report{Findings: []Finding{}}
	for _, name := range names {
		result := results.Results[name]
		if result.Status < lint.Notice {
			continue
		}
		severity := map[lint.LintStatus]string{lint.Notice: "info", lint.Warn: "warn", lint.Error: "error", lint.Fatal: "fatal"}[result.Status]
		report.addFindings(Finding{Source: "zlint", ID: name, Severity: severity, Detail: result.Details})
	}
	engine := "zlint v3"
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, dep := range info.Deps {
			if dep.Path == "github.com/zmap/zlint/v3" {
				engine = "zlint " + dep.Version
				break
			}
		}
	}
	return report.Findings, engine, nil
}
