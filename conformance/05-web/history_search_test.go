package web

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rotari-dev/rotari/conformance/support"
)

func TestMain(m *testing.M) {
	// A launched service started by TestWebLaunchServesServices.
	if os.Getenv(launchServiceRole) == "1" {
		os.Exit(serveLaunchService())
	}
	os.Exit(support.Run(m))
}

func covers(t *testing.T, _ ...string) { t.Helper() }

func TestHistorySearchAcrossProjects(t *testing.T) {
	covers(t, "WEB-2")
	e := support.NewEnv(t)
	workingDirectory := e.Root
	e.MustRotari("add", "-p", "alpha", "--", "true")
	e.MustRotari("run", "-p", "alpha", "--quiet")
	var alphaShown struct {
		RunID string `json:"run_id"`
	}
	if err := json.Unmarshal([]byte(e.MustRotari("show", "-p", "alpha", "--json").Stdout), &alphaShown); err != nil || alphaShown.RunID == "" {
		e.T.Fatalf("show --json did not describe alpha run: %v", err)
	}
	alphaRunID := alphaShown.RunID
	contextPath := filepath.Join(e.Base, "projects", "alpha", "runs", alphaRunID, "context.json")
	contextData, err := os.ReadFile(contextPath)
	if err != nil {
		t.Fatal(err)
	}
	var runContext map[string]any
	if err := json.Unmarshal(contextData, &runContext); err != nil {
		t.Fatal(err)
	}
	runContext["hostname"] = "run-host-fixture"
	contextData, err = json.Marshal(runContext)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(contextPath, contextData, 0o600); err != nil {
		t.Fatal(err)
	}
	e.FinishedJobRun("beta")
	alphaSummaryPath := filepath.Join(e.Base, "projects", "alpha", "runs", alphaRunID, "summary.json")
	alphaSummaryData, err := os.ReadFile(alphaSummaryPath)
	if err != nil {
		t.Fatal(err)
	}
	var alphaSummary map[string]any
	if err := json.Unmarshal(alphaSummaryData, &alphaSummary); err != nil {
		t.Fatal(err)
	}
	results := alphaSummary["results"].([]any)
	results[0].(map[string]any)["diagnoses"] = []map[string]string{{"name": "Permission denied"}, {"name": "Legacy custom diagnosis"}}
	results[0].(map[string]any)["hosts"] = []string{"gpu-node-07", "gpu-node-08"}
	alphaSummaryData, err = json.Marshal(alphaSummary)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(alphaSummaryPath, alphaSummaryData, 0o600); err != nil {
		t.Fatal(err)
	}
	base := e.StartWeb()
	absBase, err := filepath.Abs(e.Base)
	if err != nil {
		t.Fatal(err)
	}
	response := e.HTTPPostJSON(base+"/api/history-search", map[string]any{
		"scopes":  []map[string]string{{"basedir_id": base64.RawURLEncoding.EncodeToString([]byte(filepath.Clean(absBase)))}},
		"filters": []map[string]string{{"target": "job", "field": "command", "word": "true"}},
		"limit":   50,
	})
	if response.Status != 200 {
		t.Fatalf("history search: status %d: %s", response.Status, response.Body)
	}
	var result struct {
		Total int `json:"total"`
		Rows  []struct {
			ProjectName string `json:"project_name"`
			Target      string `json:"target"`
			JobID       string `json:"job_id"`
		} `json:"rows"`
	}
	if err := json.Unmarshal([]byte(response.Body), &result); err != nil {
		t.Fatal(err)
	}
	if result.Total != 2 || len(result.Rows) != 2 {
		t.Fatalf("history search result = %#v, want jobs from both projects", result)
	}
	projects := map[string]bool{}
	for _, row := range result.Rows {
		if row.Target != "job" || row.JobID == "" {
			t.Errorf("history search row = %#v, want a job result", row)
		}
		projects[row.ProjectName] = true
	}
	if !projects["alpha"] || !projects["beta"] {
		t.Errorf("cross-project results = %#v, want alpha and beta", projects)
	}
	searchTotal := func(target, field, word string, caseSensitive, fuzzy bool) int {
		t.Helper()
		response := e.HTTPPostJSON(base+"/api/history-search", map[string]any{
			"scopes":         []map[string]string{{"basedir_id": base64.RawURLEncoding.EncodeToString([]byte(filepath.Clean(absBase)))}},
			"target":         target,
			"case_sensitive": caseSensitive,
			"fuzzy":          fuzzy,
			"filters":        []map[string]string{{"target": target, "field": field, "word": word}},
		})
		if response.Status != 200 {
			t.Fatalf("history search for %q: status %d: %s", word, response.Status, response.Body)
		}
		var got struct {
			Total int `json:"total"`
		}
		if err := json.Unmarshal([]byte(response.Body), &got); err != nil {
			t.Fatal(err)
		}
		return got.Total
	}
	if got := searchTotal("job", "command", "TRUE", false, false); got != 2 {
		t.Errorf("default ignore-case total = %d, want 2", got)
	}
	if got := searchTotal("job", "command", "TRUE", true, false); got != 0 {
		t.Errorf("case-sensitive total = %d, want 0", got)
	}
	if got := searchTotal("job", "command", "ture", false, true); got != 2 {
		t.Errorf("fuzzy typo total = %d, want 2", got)
	}
	if got := searchTotal("job", "command", "ture", false, false); got != 0 {
		t.Errorf("exact typo total = %d, want 0", got)
	}
	if got := searchTotal("job", "host", "GPU-NODE-08", false, false); got != 1 {
		t.Errorf("host search total = %d, want 1", got)
	}
	if got := searchTotal("job", "working_directory", workingDirectory, false, false); got != 2 {
		t.Errorf("job working-directory fallback search total = %d, want 2", got)
	}
	if got := searchTotal("run", "host", "run-host-fixture", false, false); got != 1 {
		t.Errorf("run host search total = %d, want 1", got)
	}
	if got := searchTotal("run", "working_directory", workingDirectory, false, false); got != 2 {
		t.Errorf("run working-directory search total = %d, want 2", got)
	}
	searchRange := map[string]string{"basedir_id": base64.RawURLEncoding.EncodeToString([]byte(filepath.Clean(absBase)))}
	diagnosisOptions := e.HTTPGet(base + "/api/history-search-diagnoses")
	if diagnosisOptions.Status != 200 {
		t.Fatalf("diagnosis options: status %d: %s", diagnosisOptions.Status, diagnosisOptions.Body)
	}
	var options struct {
		Diagnoses []string `json:"diagnoses"`
	}
	if err := json.Unmarshal([]byte(diagnosisOptions.Body), &options); err != nil {
		t.Fatal(err)
	}
	containsDiagnosis := false
	containsLegacyDiagnosis := false
	for _, name := range options.Diagnoses {
		containsDiagnosis = containsDiagnosis || name == "Permission denied"
		containsLegacyDiagnosis = containsLegacyDiagnosis || name == "Legacy custom diagnosis"
	}
	if !containsDiagnosis || containsLegacyDiagnosis {
		t.Fatalf("diagnosis options = %#v, want standard names without saved custom labels", options.Diagnoses)
	}
	diagnosisSearch := e.HTTPPostJSON(base+"/api/history-search", map[string]any{
		"scopes":  []map[string]string{searchRange},
		"target":  "job",
		"filters": []map[string]string{{"target": "job", "field": "diagnosis", "word": "Permission denied"}},
	})
	if diagnosisSearch.Status != 200 {
		t.Fatalf("diagnosis search: status %d: %s", diagnosisSearch.Status, diagnosisSearch.Body)
	}
	var diagnosisResult struct {
		Total int `json:"total"`
	}
	if err := json.Unmarshal([]byte(diagnosisSearch.Body), &diagnosisResult); err != nil {
		t.Fatal(err)
	}
	if diagnosisResult.Total != 1 {
		t.Errorf("diagnosis search total = %d, want 1", diagnosisResult.Total)
	}
}

func TestHistorySearchFindsRecordedUnfinishedJobStatus(t *testing.T) {
	covers(t, "WEB-2")
	e := support.NewEnv(t)
	e.MustRotari("add", "-p", "demo", "--", "true")
	e.MustRotari("run", "-p", "demo", "--quiet")
	var shown struct {
		RunID string `json:"run_id"`
		Jobs  []struct {
			Job struct {
				ID string `json:"id"`
			} `json:"job"`
		} `json:"jobs"`
	}
	if err := json.Unmarshal([]byte(e.MustRotari("show", "-p", "demo", "--json").Stdout), &shown); err != nil || shown.RunID == "" || len(shown.Jobs) != 1 {
		t.Fatalf("show --json did not describe one completed job: %v", err)
	}
	jobID := shown.Jobs[0].Job.ID
	runDir := filepath.Join(e.Base, "projects", "demo", "runs", shown.RunID)
	attemptEntries, err := os.ReadDir(filepath.Join(runDir, jobID, "attempts"))
	if err != nil || len(attemptEntries) != 1 {
		t.Fatalf("attempt directory entries = %v, %v", attemptEntries, err)
	}
	attemptDir := filepath.Join(runDir, jobID, "attempts", attemptEntries[0].Name())
	for _, path := range []string{
		filepath.Join(runDir, "summary.json"),
		filepath.Join(attemptDir, "status"),
		filepath.Join(attemptDir, "finished_at"),
	} {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(attemptDir, "status.json"), []byte(`{"phase":"running"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	metaPath := filepath.Join(e.Base, "projects", "demo", "meta.json")
	if err := os.WriteFile(metaPath, []byte(`{"phase":"running","last_run_id":"`+shown.RunID+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	base := e.StartWeb()
	absBase, err := filepath.Abs(e.Base)
	if err != nil {
		t.Fatal(err)
	}
	response := e.HTTPPostJSON(base+"/api/history-search", map[string]any{
		"scopes": []map[string]string{{"basedir_id": base64.RawURLEncoding.EncodeToString([]byte(filepath.Clean(absBase))), "project_name": "demo"}},
		"target": "job", "filters": []map[string]string{{"target": "job", "field": "status", "word": "running (recorded)"}},
	})
	if response.Status != 200 {
		t.Fatalf("history search: status %d: %s", response.Status, response.Body)
	}
	var result struct {
		Total int `json:"total"`
		Rows  []struct {
			JobID     string `json:"job_id"`
			JobStatus string `json:"job_status"`
		} `json:"rows"`
	}
	if err := json.Unmarshal([]byte(response.Body), &result); err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 || len(result.Rows) != 1 || result.Rows[0].JobID != jobID || result.Rows[0].JobStatus != "running (recorded)" {
		t.Fatalf("history search = %#v, want the interrupted attempt with recorded status", result)
	}
}

// An all-basedirs scope searches every registered basedir and names the ones
// it could not search rather than failing or passing for a complete search.
func TestHistorySearchAllBasedirs(t *testing.T) {
	covers(t, "WEB-2")
	e := support.NewEnv(t)
	other := filepath.Join(e.Root, "other-state")
	gone := filepath.Join(e.Root, "gone-state")
	broken := filepath.Join(e.Root, "broken-state")
	e.MustRotari("add", "-p", "alpha", "--", "true")
	e.MustRotari("run", "-p", "alpha", "--quiet")
	for _, item := range []struct{ baseDir, project string }{{other, "beta"}, {gone, "gamma"}, {broken, "delta"}} {
		e.MustRotari("add", "-b", item.baseDir, "-p", item.project, "--", "true")
		e.MustRotari("run", "-b", item.baseDir, "-p", item.project, "--quiet")
	}
	if err := os.RemoveAll(gone); err != nil {
		t.Fatal(err)
	}
	projects := filepath.Join(broken, "projects")
	if err := os.RemoveAll(projects); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(projects, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	base := e.StartWeb()
	search := func(scopes ...map[string]any) support.HTTPResult {
		return e.HTTPPostJSON(base+"/api/history-search", map[string]any{
			"scopes":  scopes,
			"target":  "project",
			"filters": []map[string]string{{"target": "project", "field": "project_name", "word": "a"}},
		})
	}
	all := map[string]any{"all_basedirs": true}
	// The repeated scope must not search or report anything twice.
	response := search(all, all)
	if response.Status != 200 {
		t.Fatalf("history search: status %d: %s", response.Status, response.Body)
	}
	var result struct {
		Rows []struct {
			ProjectName string `json:"project_name"`
			BaseDirPath string `json:"basedir_path"`
		} `json:"rows"`
		Unsearched []struct {
			BaseDirPath string `json:"basedir_path"`
			Error       string `json:"error"`
		} `json:"unsearched"`
	}
	if err := json.Unmarshal([]byte(response.Body), &result); err != nil {
		t.Fatal(err)
	}
	found := map[string]string{}
	for _, row := range result.Rows {
		found[row.ProjectName] = row.BaseDirPath
	}
	abs := func(path string) string {
		absolute, err := filepath.Abs(path)
		if err != nil {
			t.Fatal(err)
		}
		return filepath.Clean(absolute)
	}
	if len(result.Rows) != 2 || found["alpha"] != abs(e.Base) || found["beta"] != abs(other) {
		t.Errorf("rows = %+v, want alpha and beta from their own basedirs", result.Rows)
	}
	reasons := map[string]string{}
	for _, item := range result.Unsearched {
		reasons[item.BaseDirPath] = item.Error
	}
	if len(result.Unsearched) != 2 || reasons[abs(gone)] != "basedir does not exist" || reasons[abs(broken)] == "" {
		t.Errorf("unsearched = %+v, want the removed and the unreadable basedir", result.Unsearched)
	}
	for _, scope := range []map[string]any{
		{"all_basedirs": true, "basedir_id": "x"},
		{"all_basedirs": true, "project_name": "alpha"},
		{"all_basedirs": true, "run_id": "x"},
	} {
		if response := search(scope); response.Status == 200 {
			t.Errorf("scope %v: status 200, want it rejected: %s", scope, response.Body)
		}
	}
}

func TestHistorySearchOfRemovedBasedirFails(t *testing.T) {
	covers(t, "WEB-2")
	e := support.NewEnv(t)
	gone := filepath.Join(e.Root, "gone-state")
	e.MustRotari("add", "-b", gone, "-p", "gamma", "--", "true")
	e.MustRotari("run", "-b", gone, "-p", "gamma", "--quiet")
	absolute, err := filepath.Abs(gone)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(gone); err != nil {
		t.Fatal(err)
	}
	id := base64.RawURLEncoding.EncodeToString([]byte(filepath.Clean(absolute)))
	base := e.StartWeb()
	for _, scope := range []map[string]string{
		{"basedir_id": id},
		{"basedir_id": id, "project_name": "default"},
	} {
		response := e.HTTPPostJSON(base+"/api/history-search", map[string]any{
			"scopes":  []map[string]string{scope},
			"target":  "project",
			"filters": []map[string]string{{"target": "project", "field": "project_name", "word": "default"}},
		})
		if response.Status == 200 || !strings.Contains(response.Body, "basedir does not exist") {
			t.Errorf("scope %v: status %d: %s, want the missing basedir reported", scope, response.Status, response.Body)
		}
	}
	for _, query := range []string{"basedir_id=" + id, "basedir_id=" + id + "&project_name=default"} {
		response := e.HTTPGet(base + "/api/history-search-options?" + query)
		if response.Status == 200 || !strings.Contains(response.Body, "basedir does not exist") {
			t.Errorf("options %s: status %d: %s, want the missing basedir reported", query, response.Status, response.Body)
		}
	}
}
