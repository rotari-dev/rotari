package webui

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// The Artifacts button and the attempt menu pass a job's identity to their
// handlers, whatever characters it contains. A browser decodes an attribute
// before parsing it as JavaScript, so HTML escaping alone would let a quote in
// a project name end the handler's argument and run what follows it.
func TestWebJobActionHandlersKeepIdentityAsData(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is not installed")
	}
	htmlPath := filepath.Join(t.TempDir(), "index.html")
	if err := os.WriteFile(htmlPath, []byte(testSite().webHTML()), 0o600); err != nil {
		t.Fatal(err)
	}
	script := `
const fs = require('fs');
const assert = require('node:assert/strict');
const { JSDOM, VirtualConsole } = require('jsdom');
const hostile = name => name + "');window.__injected=1;('\"<b>&amp;";
const project = hostile('project'), runID = hostile('run'), jobID = hostile('job');
const latest = hostile('latest'), older = hostile('older');
const result = { exit_code: 0 };
const run = {
  run_id: runID, status: 'success', lifecycle: 'finished', started_at: '2026-10-11 05:52:20 JST',
  jobs: [{
    id: jobID, name: 'decode', command: ['true'], executor: 'local', working_directory: '/work',
    attempt_id: latest, submitted_at: '2026-10-11 05:52:22 JST', finished_at: '2026-10-11 05:55:27 JST',
    elapsed: '3m 05s', result,
    attempts: [{ id: latest, result }, { id: older, result }],
  }],
};
const state = { base_dir: '/state', projects: [{ project_name: project, queue: { commands: [] }, runs: [run] }] };
const errors = [];
const virtualConsole = new VirtualConsole();
virtualConsole.on('jsdomError', error => errors.push(error.stack || String(error)));
const dom = new JSDOM(fs.readFileSync(process.argv[1], 'utf8'), {
  runScripts: 'dangerously',
  url: 'http://127.0.0.1/project/' + encodeURIComponent(project) + '/run/' + encodeURIComponent(runID),
  virtualConsole,
  beforeParse(window) {
    window.setInterval = () => 1;
    window.fetch = async (url) => {
      if (url === '/api/state') return { ok: true, json: async () => state };
      if (url === '/api/launchers') return { ok: true, json: async () => ({ available: false }) };
      return { ok: false, json: async () => null, text: async () => '' };
    };
  },
});
setTimeout(() => {
  try {
    const window = dom.window, document = window.document;
    const calls = [];
    window.showArtifacts = (...args) => calls.push(['showArtifacts', ...args]);
    window.selectJobAttempt = (...args) => calls.push(['selectJobAttempt', ...args]);

    const artifacts = document.querySelector('button.view-artifacts');
    assert(artifacts, 'the Artifacts button was not rendered');
    artifacts.click();
    const attempts = [...document.querySelectorAll('.attempt-menu button')];
    assert.equal(attempts.length, 2, 'the attempt menu does not list both attempts');
    attempts[1].click();

    assert.equal(window.__injected, undefined, 'a value ran as code');
    assert.deepEqual(calls, [
      ['showArtifacts', project, runID, jobID, latest],
      ['selectJobAttempt', project, runID, jobID, older],
    ]);
    if (errors.length) throw new Error(errors.join('\n'));
  } catch (error) {
    console.error(error.stack || String(error));
    process.exit(1);
  }
  process.exit(0);
}, 100);
`
	if output, err := exec.Command("node", "-e", script, htmlPath).CombinedOutput(); err != nil {
		t.Fatalf("job action handlers check failed: %v\n%s", err, output)
	}
}
