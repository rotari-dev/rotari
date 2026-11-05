package web

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rotari-dev/rotari/conformance/support"
)

func TestTableViewerExportsRows(t *testing.T) {
	covers(t, "WEB-31")
	e := support.NewEnv(t)
	long := strings.Repeat("x", 5000)
	jsonLines := `{"id": 1, "score": 1.50, "extra": null}` + "\n\n" + `{"id": 2, "score": 3, "nested": {"k": [1,  2]}}` + "\n"
	files := map[string]string{
		"data.csv":   "id,,note\n1,3,\"a,b\"\n2,1\n3,2," + long + "\n",
		"data.tsv":   "id\tnote\n1\ta,b\n",
		"data.jsonl": jsonLines,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(e.Root, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	addresses := map[string]string{}
	for name := range files {
		addresses[name] = strings.TrimSuffix(e.StartTableView(filepath.Join(e.Root, name)), "/")
	}
	for _, test := range []struct {
		name, file, want string
		query            url.Values
		status           int
	}{
		{
			name: "all rows with the header as written", file: "data.csv", status: 200,
			query: url.Values{"sep": {","}},
			want:  files["data.csv"],
		},
		{
			name: "no header", file: "data.csv", status: 200,
			query: url.Values{"sep": {","}, "header": {"0"}},
			want:  files["data.csv"],
		},
		{
			name: "filters and sort, with a whole long cell", file: "data.csv", status: 200,
			query: url.Values{"sep": {","}, "filters": {`[{"column":1,"op":">","value":"1"}]`}, "sort": {"1"}, "desc": {"1"}},
			want:  "id,,note\n1,3,\"a,b\"\n3,2," + long + "\n",
		},
		{
			name: "no rows selected", file: "data.csv", status: 200,
			query: url.Values{"sep": {","}, "filters": {`[{"column":0,"op":"=","value":"none"}]`}},
			want:  "id,,note\n",
		},
		{
			name: "its own separator", file: "data.tsv", status: 200,
			query: url.Values{"sep": {"\t"}},
			want:  files["data.tsv"],
		},
		{
			name: "JSON Lines keeps its lines", file: "data.jsonl", status: 200,
			query: url.Values{},
			want:  `{"id": 1, "score": 1.50, "extra": null}` + "\n" + `{"id": 2, "score": 3, "nested": {"k": [1,  2]}}` + "\n",
		},
		{name: "offset", file: "data.csv", status: 400, query: url.Values{"sep": {","}, "offset": {"1"}}},
		{name: "limit", file: "data.csv", status: 400, query: url.Values{"sep": {","}, "limit": {"1"}}},
		{name: "base", file: "data.csv", status: 400, query: url.Values{"sep": {","}, "base": {"file"}}},
		{name: "bad filter", file: "data.csv", status: 400, query: url.Values{"sep": {","}, "filters": {`[{"column":9,"op":"="}]`}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := e.HTTPGet(addresses[test.file] + "/api/export?" + test.query.Encode())
			if response.Status != test.status {
				t.Fatalf("status = %d: %s", response.Status, response.Body)
			}
			if test.status == 200 && response.Body != test.want {
				t.Fatalf("body = %q, want %q", response.Body, test.want)
			}
		})
	}
	// A directory's listing, as CSV.
	address := strings.TrimSuffix(e.StartTableView(e.Root), "/")
	response := e.HTTPGet(address + "/api/export?" + url.Values{"pattern": {"data.*"}}.Encode())
	if response.Status != 200 || !strings.HasPrefix(response.Body, "path,") || !strings.Contains(response.Body, "\ndata.csv,") {
		t.Fatalf("directory = (%d, %q)", response.Status, response.Body)
	}
}
