package webui

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rotari-dev/rotari/internal/model"
	stateinternal "github.com/rotari-dev/rotari/internal/state"
)

// interruptedProject writes a project whose metadata still records run-1 as
// running, with no lock and no coordinator record: unlock can only release
// it, and only on the user's word that it stopped.
func interruptedProject(t *testing.T) (string, stateinternal.ProjectPaths) {
	t.Helper()
	baseDir := t.TempDir()
	paths, err := stateinternal.ResolveProjectPaths(baseDir, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(paths.RunsDir, "run-1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := stateinternal.WriteJSON(paths.MetaFile, model.Meta{Phase: "running", LastRunID: "run-1"}); err != nil {
		t.Fatal(err)
	}
	return baseDir, paths
}

func unlockRequest(t *testing.T, handler http.Handler, method, target string, body any) (int, webUnlockResult, string) {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		reader = bytes.NewReader(data)
	} else {
		reader = bytes.NewReader(nil)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(method, target, reader))
	var result webUnlockResult
	_ = json.Unmarshal(recorder.Body.Bytes(), &result)
	return recorder.Code, result, recorder.Body.String()
}

// The Web previews and applies unlock as the CLI and MCP do: a preview
// explains a refusal, an apply needs the preview's revision and, for
// unverifiable work, the user's attestation; refusals and stale revisions
// answer 409 and change nothing.
func TestWebUnlockPreviewsAndAppliesThroughProjectUnlock(t *testing.T) {
	baseDir, paths := interruptedProject(t)
	handler := Handler(testOptions(baseDir, true))

	code, preview, body := unlockRequest(t, handler, http.MethodGet, "/api/unlock?project_name=demo", nil)
	if code != http.StatusOK || preview.RunID != "run-1" || preview.Action != "release" || preview.Allowed || preview.RefusalCode != "coordinator_unknown" || preview.Refusal == "" {
		t.Fatalf("preview = %d %s", code, body)
	}
	if !strings.Contains(preview.RerunCommand, "rotari retry") || !strings.Contains(preview.RerunCommand, "run-1") {
		t.Fatalf("rerun command = %q", preview.RerunCommand)
	}
	code, attested, body := unlockRequest(t, handler, http.MethodGet, "/api/unlock?project_name=demo&assume_stopped=true", nil)
	if code != http.StatusOK || !attested.Allowed || attested.Refusal != "" {
		t.Fatalf("attested preview = %d %s", code, body)
	}

	readOnly := testOptions(baseDir, false)
	for name, test := range map[string]struct {
		handler http.Handler
		body    map[string]any
		code    int
	}{
		"read-only":       {Handler(readOnly), map[string]any{"project_name": "demo", "if_revision": attested.Revision, "assume_stopped": true}, http.StatusForbidden},
		"no revision":     {handler, map[string]any{"project_name": "demo", "assume_stopped": true}, http.StatusBadRequest},
		"stale revision":  {handler, map[string]any{"project_name": "demo", "if_revision": "stale", "assume_stopped": true}, http.StatusConflict},
		"no attestation":  {handler, map[string]any{"project_name": "demo", "if_revision": preview.Revision}, http.StatusConflict},
		"backslash name":  {handler, map[string]any{"project_name": `de\mo`, "if_revision": attested.Revision}, http.StatusBadRequest},
		"missing project": {handler, map[string]any{"project_name": "other", "if_revision": attested.Revision}, http.StatusBadRequest},
	} {
		code, result, body := unlockRequest(t, test.handler, http.MethodPost, "/api/unlock", test.body)
		if code != test.code {
			t.Fatalf("%s: %d %s", name, code, body)
		}
		if name == "no attestation" && result.RefusalCode != "coordinator_unknown" {
			t.Fatalf("%s: result = %s", name, body)
		}
		if meta, err := stateinternal.LoadMeta(paths.MetaFile); err != nil || meta.Phase != "running" {
			t.Fatalf("%s changed the project: %+v, %v", name, meta, err)
		}
	}

	code, applied, body := unlockRequest(t, handler, http.MethodPost, "/api/unlock", map[string]any{"project_name": "demo", "run_id": "run-1", "if_revision": attested.Revision, "assume_stopped": true})
	if code != http.StatusOK || !applied.Applied || applied.Action != "release" || applied.RerunCommand == "" {
		t.Fatalf("apply = %d %s", code, body)
	}
	if meta, err := stateinternal.LoadMeta(paths.MetaFile); err != nil || meta.Phase != "collecting" {
		t.Fatalf("project after release = %+v, %v", meta, err)
	}
	if code, idle, body := unlockRequest(t, handler, http.MethodGet, "/api/unlock?project_name=demo", nil); code != http.StatusOK || idle.RunID != "" || !idle.Allowed {
		t.Fatalf("preview after release = %d %s", code, body)
	}
}

// The Recover dialog shows the preview and applies only what it allows: live
// work keeps it refused, unverifiable work needs the user's tick, which is
// sent as assume_stopped with the preview's revision, and a conflicting apply
// asks for a new preview. Interrupted runs get the button on both pages.
func TestRecoverDialogFollowsThePreview(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is not installed")
	}
	script := `
const fs = require('fs');
const { JSDOM, VirtualConsole } = require('jsdom');
const errors = [];
const virtualConsole = new VirtualConsole();
virtualConsole.on('jsdomError', error => errors.push(error.stack || String(error)));
const previews = {
  active: () => ({run_id: 'run-1', action: 'release', allowed: false, refusal_code: 'execution_active', refusal: 'execution_active: stop the work', revision: 'r1',
    coordinator: {state: 'stopped', reason: 'process exited'}, repair: {reason: 'no complete summary', executions: [{job_id: 'train', state: 'active', reason: 'process group alive', source: 'local_process_group'}]}}),
  unknown: assume => assume
    ? {run_id: 'run-1', action: 'release', allowed: true, revision: 'r2', assume_stopped: true, coordinator: {state: 'unknown', reason: 'another host'}, repair: {executions: []}}
    : {run_id: 'run-1', action: 'release', allowed: false, refusal_code: 'coordinator_unknown', refusal: 'coordinator_unknown: confirm it stopped', revision: 'r1', coordinator: {state: 'unknown', reason: 'another host'}, repair: {executions: []}},
  finalize: () => ({run_id: 'run-1', action: 'finalize', allowed: true, revision: 'r3', coordinator: {state: 'stopped', reason: 'process exited'}, repair: {complete: true, executions: []}}),
};
let scenario = 'active';
const requests = [];
const posts = [];
const dom = new JSDOM(fs.readFileSync(process.argv[1], 'utf8'), {
  runScripts: 'dangerously', url: 'http://127.0.0.1/', virtualConsole,
  beforeParse(window) {
    window.setInterval = () => 1;
    window.fetch = async (url, init) => {
      url = String(url);
      if (url.startsWith('/api/unlock?')) {
        requests.push(url);
        const assume = new URLSearchParams(url.split('?')[1]).get('assume_stopped') === 'true';
        return {ok: true, json: async () => previews[scenario](assume)};
      }
      if (url === '/api/unlock') {
        posts.push(JSON.parse(init.body));
        if (scenario === 'finalize') return {ok: false, status: 409, text: async () => JSON.stringify({error: 'project changed since the planned revision'})};
        return {ok: true, text: async () => JSON.stringify({run_id: 'run-1', action: 'release', applied: true, rerun_command: 'rotari retry --run-id run-1'})};
      }
      if (url.startsWith('/api/notifications')) return {ok: true, json: async () => ({entries: [], basedirs: {}})};
      return {ok: true, json: async () => ({base_dir: '/state', projects: []})};
    };
  },
});
const settle = () => new Promise(resolve => setTimeout(resolve, 30));
(async () => {
  try {
    await settle();
    const w = dom.window, d = w.document;
    const view = () => d.getElementById('recover-view');
    const apply = () => view().querySelector('.recover-apply');
    if (!w.runActionsHTML({project_name: 'demo'}, {lifecycle: 'interrupted', context: {}}, 'run-1').includes('Recover'))
      throw Error('no Recover on an interrupted run page');
    if (w.runActionsHTML({project_name: 'demo'}, {lifecycle: 'finished', context: {}}, 'run-1').includes('Recover'))
      throw Error('Recover on a finished run');
    if (!w.projectRunsHTML({project_name: 'demo', runs: [{run_id: 'run-1', lifecycle: 'interrupted'}]}).includes('Recover'))
      throw Error('no Recover on the project page');

    await w.showRecoverRun('demo', 'run-1');
    if (view().hidden || !apply().disabled || view().querySelector('.recover-assume-stopped') || !view().textContent.includes('stop the work') || !view().textContent.includes('train'))
      throw Error('live work was not refused: ' + view().innerHTML);

    scenario = 'unknown';
    await w.showRecoverRun('demo', 'run-1');
    const tick = view().querySelector('.recover-assume-stopped');
    if (!apply().disabled || !tick || tick.checked) throw Error('unverified work was not held for attestation');
    tick.checked = true;
    tick.dispatchEvent(new w.Event('change'));
    await settle();
    if (!requests.at(-1).includes('assume_stopped=true') || apply().disabled) throw Error('the tick did not re-preview: ' + requests.at(-1));
    apply().click();
    await settle();
    const sent = posts.at(-1);
    if (sent.if_revision !== 'r2' || sent.assume_stopped !== true || sent.project_name !== 'demo' || sent.run_id !== 'run-1')
      throw Error('apply sent ' + JSON.stringify(sent));
    if (!view().querySelector('.recover-outcome').textContent.includes('rotari retry --run-id run-1')) throw Error('no rerun command');

    scenario = 'finalize';
    await w.showRecoverRun('demo', 'run-1');
    if (apply().disabled || apply().textContent !== 'Finalize run') throw Error('finalize not offered');
    apply().click();
    await settle();
    const outcome = view().querySelector('.recover-outcome').textContent;
    if (!outcome.includes('project changed') || !outcome.includes('Preview again')) throw Error('conflict not explained: ' + outcome);
    if (posts.at(-1).assume_stopped !== false) throw Error('attestation sent without the tick');
    if (errors.length) throw Error(errors.join('\n'));
    dom.window.close();
  } catch (error) { console.error(error); process.exit(1); }
})();
`
	path := filepath.Join(t.TempDir(), "index.html")
	if err := os.WriteFile(path, []byte(siteFor("/state").webHTML()), 0o600); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("node", "-e", script, path).CombinedOutput(); err != nil {
		t.Fatalf("recover dialog: %v\n%s", err, output)
	}
}
