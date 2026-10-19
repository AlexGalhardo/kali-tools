package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCLIContract(t *testing.T) {
	for _, test := range []struct {
		args      []string
		code      int
		errorCode string
	}{{[]string{"version"}, 0, ""}, {[]string{"interfaces"}, 0, ""}, {[]string{"diagnose", "--target", "a;whoami"}, 2, "invalid_input"}, {[]string{"discover", "--target", "0.0.0.0/0"}, 2, "invalid_input"}, {[]string{"unknown"}, 2, "invalid_input"}} {
		var out, stderr bytes.Buffer
		code := run(t.Context(), test.args, &out, &stderr)
		if code != test.code {
			t.Fatal(test.args, code, out.String(), stderr.String())
		}
		var response envelope
		if e := json.Unmarshal(out.Bytes(), &response); e != nil {
			t.Fatal(e, out.String())
		}
		if response.SchemaVersion != 1 {
			t.Fatal(response)
		}
		if test.errorCode != "" && (response.Error == nil || response.Error.Code != test.errorCode) {
			t.Fatal(response)
		}
	}
}
func TestCLIHTTPAndMalformedFlags(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	defer server.Close()
	var out, stderr bytes.Buffer
	if code := run(t.Context(), []string{"diagnose", "--kind", "http", "--target", server.URL}, &out, &stderr); code != 0 {
		t.Fatal(code, out.String())
	}
	if !bytes.Contains(out.Bytes(), []byte("204 No Content")) {
		t.Fatal(out.String())
	}
	if code := run(t.Context(), []string{"diagnose", "--timeout", "NaN"}, &out, &stderr); code != 2 {
		t.Fatal(code)
	}
}
