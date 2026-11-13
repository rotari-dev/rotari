package webui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestShutdownRoute checks POST /_server/shutdown's guards, that it answers before asking
// for the shutdown, and that pages offer Stop server only when it would work.
func TestShutdownRoute(t *testing.T) {
	called := make(chan struct{}, 4)
	stop := func() { called <- struct{}{} }
	request := func(options Options, method, contentType string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, "http://localhost"+serverShutdownPath, strings.NewReader("{}"))
		if contentType != "" {
			r.Header.Set("Content-Type", contentType)
		}
		w := httptest.NewRecorder()
		Handler(options).ServeHTTP(w, r)
		return w
	}
	live := testOptions(t.TempDir(), true)
	live.Shutdown = stop
	readOnly := testOptions(t.TempDir(), false)
	readOnly.Shutdown = stop
	for _, c := range []struct {
		name        string
		options     Options
		method      string
		contentType string
		want        int
	}{
		{"no shutdown wired", testOptions(t.TempDir(), true), http.MethodPost, "application/json", http.StatusNotFound},
		{"GET", live, http.MethodGet, "", http.StatusMethodNotAllowed},
		{"read-only", readOnly, http.MethodPost, "application/json", http.StatusForbidden},
		{"form post", live, http.MethodPost, "application/x-www-form-urlencoded", http.StatusUnsupportedMediaType},
		{"text post", live, http.MethodPost, "text/plain", http.StatusUnsupportedMediaType},
	} {
		if got := request(c.options, c.method, c.contentType).Code; got != c.want {
			t.Errorf("%s: status %d, want %d", c.name, got, c.want)
		}
	}
	select {
	case <-called:
		t.Fatal("a refused request stopped the server")
	default:
	}
	if w := request(live, http.MethodPost, "application/json; charset=utf-8"); w.Code != http.StatusAccepted {
		t.Fatalf("JSON POST: status %d: %s", w.Code, w.Body)
	}
	select {
	case <-called:
	case <-time.After(5 * time.Second):
		t.Fatal("an accepted request did not stop the server")
	}

	page := func(options Options) string {
		t.Helper()
		w := httptest.NewRecorder()
		Handler(options).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "http://localhost/", nil))
		return w.Body.String()
	}
	if !strings.Contains(page(live), `id="stop-server-button"`) {
		t.Error("a live page with job control lacks Stop server")
	}
	for name, options := range map[string]Options{"read-only": readOnly, "no shutdown": testOptions(t.TempDir(), true)} {
		if strings.Contains(page(options), `id="stop-server-button"`) {
			t.Errorf("%s page offers Stop server", name)
		}
	}
	if strings.Contains(page(live), "__ROTARI_STOP_SERVER__") {
		t.Error("the placeholder was left in the page")
	}
}
