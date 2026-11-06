package webui

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

type authEventRecorder struct {
	events []AuthEvent
}

func TestAuthEvidenceDistinguishesMissingSessionsAndRefusals(t *testing.T) {
	now := time.Now()
	sessions := &webSessions{expiries: map[string]time.Time{"expired": now.Add(-time.Second), "current": now.Add(time.Hour)}}
	request := httptest.NewRequest(http.MethodGet, "https://localhost/", nil)
	if got := sessions.decision(request, now); got != sessionAbsent {
		t.Errorf("no cookie = %q", got)
	}
	request.AddCookie(&http.Cookie{Name: webSessionCookie, Value: "unknown"})
	if got := sessions.decision(request, now); got != sessionUnknown {
		t.Errorf("unknown cookie = %q", got)
	}
	request = httptest.NewRequest(http.MethodGet, "https://localhost/", nil)
	request.AddCookie(&http.Cookie{Name: webSessionCookie, Value: "expired"})
	if got := sessions.decision(request, now); got != sessionExpired {
		t.Errorf("expired cookie = %q", got)
	}

	recorder := &authEventRecorder{}
	handler := WithAuthToken(http.NotFoundHandler(), "secret", AuthOptions{Recorder: recorder})
	refusedLogin := httptest.NewRequest(http.MethodGet, "http://localhost/_auth/login", nil)
	refusedLogin.Header.Set("User-Agent", "browser")
	handler.ServeHTTP(httptest.NewRecorder(), refusedLogin)
	refusedPage := httptest.NewRequest(http.MethodGet, "http://localhost/private", nil)
	refusedPage.Header.Set("Accept", "text/html")
	handler.ServeHTTP(httptest.NewRecorder(), refusedPage)
	rejectedCredential := httptest.NewRequest(http.MethodGet, "https://localhost/api/state", nil)
	rejectedCredential.Header.Set("Authorization", "Bearer wrong")
	handler.ServeHTTP(httptest.NewRecorder(), rejectedCredential)
	for index, want := range []struct{ event, cookie string }{
		{"sign_in_refused", ""}, {"session_missing", "transport_refused"}, {"credential_rejected", ""},
	} {
		if got := recorder.events[index]; got.Event != want.event || got.Cookie != want.cookie {
			t.Errorf("event %d = %#v, want %s cookie=%s", index, got, want.event, want.cookie)
		}
	}
}

func (recorder *authEventRecorder) RecordAuthEvent(event AuthEvent) {
	recorder.events = append(recorder.events, event)
}

func TestAuthRecordsSignInAndMissingSessionEvidence(t *testing.T) {
	recorder := &authEventRecorder{}
	handler := WithAuthToken(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), "secret", AuthOptions{Recorder: recorder})

	post := func(username, password, origin string) *httptest.ResponseRecorder {
		form := url.Values{"username": {username}, "password": {password}, "next": {"/private-marker"}}
		request := httptest.NewRequest(http.MethodPost, "https://localhost/_auth/login?query-marker", strings.NewReader(form.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.Header.Set("Origin", origin)
		request.Header.Set("User-Agent", "Browser marker\nforged-record")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}
	post("token-in-wrong-field", "wrong-password", "https://localhost")
	post("rotari", "secret", "https://attacker.example")
	accepted := post("rotari", "secret", "https://localhost")

	missing := httptest.NewRequest(http.MethodGet, "https://localhost/private-path-marker", nil)
	missing.Header.Set("Accept", "text/html")
	missing.Header.Set("User-Agent", strings.Repeat("u", 300))
	handler.ServeHTTP(httptest.NewRecorder(), missing)
	logout := httptest.NewRequest(http.MethodPost, "https://localhost/_auth/logout", nil)
	logout.Header.Set("Origin", "https://localhost")
	logout.AddCookie(accepted.Result().Cookies()[0])
	handler.ServeHTTP(httptest.NewRecorder(), logout)

	if len(recorder.events) != 5 {
		t.Fatalf("events = %#v, want three sign-in decisions, one missing session, and sign-out", recorder.events)
	}
	for index, want := range []struct{ event, outcome string }{
		{"sign_in", "wrong_credentials"},
		{"sign_in", "cross_origin"},
		{"sign_in", "accepted"},
		{"session_missing", ""},
		{"sign_out", ""},
	} {
		if got := recorder.events[index]; got.Event != want.event || got.Outcome != want.outcome {
			t.Errorf("event %d = %#v, want %s/%s", index, got, want.event, want.outcome)
		}
	}
	if got := recorder.events[2]; got.Transport != "https" || got.CookieSecure == nil || !*got.CookieSecure {
		t.Errorf("accepted event = %#v, want HTTPS with a Secure cookie", got)
	}
	if got := recorder.events[3]; got.Cookie != "absent" || len(got.UserAgent) != 256 {
		t.Errorf("missing-session event = %#v, want absent cookie and bounded user agent", got)
	}
	if text := fmt.Sprint(recorder.events); strings.Contains(text, "wrong-password") || strings.Contains(text, "token-in-wrong-field") || strings.Contains(text, "private-marker") {
		t.Fatalf("authentication evidence contains a credential or URL marker: %q", text)
	}
}
