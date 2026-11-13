package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestHealth(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()

	newHandler(false).ServeHTTP(response, request)

	if response.Code != http.StatusOK || response.Body.String() != "ok\n" {
		t.Fatalf("unexpected response: status=%d body=%q", response.Code, response.Body.String())
	}
}

func TestUnsafeEndpointsDisabledByDefault(t *testing.T) {
	for _, path := range []string{"/read?path=/etc/passwd", "/exec?cmd=id"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		newHandler(false).ServeHTTP(response, request)
		if response.Code != http.StatusForbidden {
			t.Errorf("%s: got status %d, want %d", path, response.Code, http.StatusForbidden)
		}
	}
}

func TestVulnerableFileRead(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "secret-")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("conference-secret\n"); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodGet, "/read?path="+file.Name(), nil)
	response := httptest.NewRecorder()
	newHandler(true).ServeHTTP(response, request)

	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "conference-secret") {
		t.Fatalf("unexpected response: status=%d body=%q", response.Code, response.Body.String())
	}
}
