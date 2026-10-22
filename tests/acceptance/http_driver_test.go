package acceptance

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/adapter/contract"
	"github.com/AlexGalhardo/kali-tools/internal/adapter/httpapi"
	"github.com/AlexGalhardo/kali-tools/internal/app"
)

type httpDriver struct {
	t      *testing.T
	client *http.Client
	url    string
}

func newHTTP(t *testing.T, a *app.Application) Driver {
	t.Helper()
	s, err := httpapi.New(a, httpapi.Options{
		Addr:   "127.0.0.1:0",
		Assets: fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("fixture")}},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(3 * time.Second):
			t.Error("HTTP acceptance server did not stop")
		}
	})
	launch, err := url.Parse(s.URL())
	if err != nil {
		t.Fatal(err)
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	d := httpDriver{t: t, client: &http.Client{Jar: jar, Timeout: 3 * time.Second}, url: launch.Scheme + "://" + launch.Host}
	t.Cleanup(d.client.CloseIdleConnections)
	_, err = httpResult[struct{}](d, testContext(t), "POST", "/api/session", map[string]string{
		"token": strings.TrimPrefix(launch.Fragment, "token="),
	})
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func (httpDriver) Name() string { return "http" }

func httpResult[T any](d httpDriver, ctx context.Context, method, path string, body any) (T, error) {
	d.t.Helper()
	var result T
	var input io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			d.t.Fatal(err)
		}
		input = bytes.NewReader(raw)
	}
	r, err := http.NewRequestWithContext(ctx, method, d.url+path, input)
	if err != nil {
		d.t.Fatal(err)
	}
	if body != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	response, err := d.client.Do(r)
	if err != nil {
		return result, err
	}
	defer response.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(response.Body, 1<<20))
	if response.StatusCode >= 400 {
		var value struct {
			Error *contract.Error `json:"error"`
		}
		if err := decoder.Decode(&value); err != nil || value.Error == nil {
			d.t.Fatalf("HTTP %d: invalid error envelope: %v", response.StatusCode, err)
		}
		return result, value.Error
	}
	if response.StatusCode != http.StatusNoContent {
		if err := decoder.Decode(&result); err != nil {
			d.t.Fatal(err)
		}
	}
	return result, nil
}

func (d httpDriver) Diagnose(ctx context.Context, r contract.DiagnosticRequest) (contract.DiagnosticResult, error) {
	return httpResult[contract.DiagnosticResult](d, ctx, "POST", "/api/diagnose", r)
}

func (d httpDriver) Discover(ctx context.Context, cidr string, ports []int, _ time.Duration) ([]contract.PortResult, error) {
	return httpResult[[]contract.PortResult](d, ctx, "POST", "/api/discovery", struct {
		CIDR  string `json:"cidr"`
		Ports []int  `json:"ports"`
	}{CIDR: cidr, Ports: ports})
}

func (d httpDriver) History(ctx context.Context) ([]contract.Job, error) {
	return httpResult[[]contract.Job](d, ctx, "GET", "/api/history", nil)
}

func (d httpDriver) ClearHistory(ctx context.Context) error {
	_, err := httpResult[struct{}](d, ctx, "DELETE", "/api/history", nil)
	return err
}
