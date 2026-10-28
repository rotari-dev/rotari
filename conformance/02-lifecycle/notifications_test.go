package lifecycle

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/rotari-dev/rotari/conformance/support"
)

func TestWebhookJobNotificationLinksToExactJob(t *testing.T) {
	covers(t, "NOT-1")
	e := support.NewEnv(t)
	payloads := make(chan struct {
		Events []struct {
			Event  string `json:"event"`
			Fields []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"fields"`
		} `json:"events"`
	}, 1)
	webhook := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var payload struct {
			Events []struct {
				Event  string `json:"event"`
				Fields []struct {
					Name  string `json:"name"`
					Value string `json:"value"`
				} `json:"fields"`
			} `json:"events"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Errorf("decode webhook payload: %v", err)
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		payloads <- payload
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer webhook.Close()

	settings := "[webhook]\nurl = " + strconv.Quote(webhook.URL) +
		"\nweb_url = \"https://rotari.example.test/rotari\"\nformat = \"json\"\nfields = [\"link\"]\n"
	if err := os.MkdirAll(e.Base, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(e.Base, "notifications.toml"), []byte(settings), 0o600); err != nil {
		t.Fatal(err)
	}
	jobID := support.AddedJobID(t, e.MustRotari("add", "-p", "p1", "--job-name", "fail", "--", "sh", "-c", "exit 1"))
	if run := e.RotariRun("run", "-p", "p1", "--quiet"); run.Code == 0 {
		t.Fatalf("failing run unexpectedly succeeded: %s", run)
	}
	summary := readSummary(t, e, "p1")

	payload := <-payloads
	for _, event := range payload.Events {
		if event.Event != "job.finished" {
			continue
		}
		for _, field := range event.Fields {
			if field.Name == "link" {
				// web_url is the server root; the link mounts the run's own basedir.
				base, err := filepath.Abs(e.Base)
				if err != nil {
					t.Fatal(err)
				}
				want := "https://rotari.example.test/rotari/_basedir/" + base64.RawURLEncoding.EncodeToString([]byte(base)) +
					"/project/p1/run/" + summary.RunID + "?job_id=" + url.QueryEscape(jobID)
				if field.Value != want {
					t.Fatalf("job link = %q, want %q", field.Value, want)
				}
				return
			}
		}
		t.Fatalf("job event has no link: %#v", event)
	}
	t.Fatalf("webhook has no job.finished event: %#v", payload.Events)
}
