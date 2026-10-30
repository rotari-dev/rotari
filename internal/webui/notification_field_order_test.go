package webui

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"

	"github.com/rotari-dev/rotari/internal/notification"
)

func TestNotificationConfigEditorOrdersFields(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is not installed")
	}
	start := strings.Index(webAppCoreJS, "function notificationFieldControl(")
	end := strings.Index(webAppCoreJS, "async function generateNotificationConfig(")
	if start < 0 || end <= start {
		t.Fatal("notification editor functions were not found in web_app_core.js")
	}
	code, _ := json.Marshal(webAppCoreJS[start:end])
	fields, _ := json.Marshal(notification.AvailableFields())
	script := `
const { JSDOM } = require('jsdom');
const { window } = new JSDOM('<form id="form"></form>', { runScripts: 'outside-only' });
window.eval(` + string(code) + `);
const { document, notificationChannelEditor, initializeNotificationConfigDirtyState, readNotificationChannel } = window;
const fields = ` + string(fields) + `;
const fail = (code, detail) => { console.error(code, JSON.stringify(detail)); process.exit(code); };
const form = document.getElementById('form');
const settings = { job_failure: true, job_success: false, run_failure: true, run_success: true, max_jobs: 10,
  fields: ['attempt_id', 'job_name', 'project'] };
form.append(notificationChannelEditor('browser', settings, fields));
initializeNotificationConfigDirtyState(form);
const saved = () => readNotificationChannel(form, 'browser').fields;
const rows = () => [...form.querySelectorAll('.notification-field-order li input')].map(input => input.value);
const move = (field, delta) => form.querySelector('.notification-field-order li input[value="' + field + '"]')
  .closest('li').querySelector('.notification-field-move[data-delta="' + delta + '"]').click();
const dirty = () => form.classList.contains('dirty');

// A saved order is shown and saved as it is, with the unselected fields after it.
if (JSON.stringify(rows()) !== JSON.stringify(settings.fields)) fail(1, rows());
if (JSON.stringify(saved()) !== JSON.stringify(settings.fields)) fail(2, saved());
const unselected = [...form.querySelectorAll('.notification-field-grid input')].map(input => input.value);
if (JSON.stringify(unselected) !== JSON.stringify(fields.filter(field => !settings.fields.includes(field)))) fail(3, unselected);
if (dirty()) fail(4);
const first = form.querySelector('.notification-field-order li');
if (!first.querySelector('[data-delta="-1"]').disabled || first.querySelector('[data-delta="1"]').disabled) fail(5);

// Moving a field changes the saved order and marks the form dirty.
move('attempt_id', 1);
if (JSON.stringify(saved()) !== JSON.stringify(['job_name', 'attempt_id', 'project'])) fail(6, saved());
if (!dirty()) fail(7);
move('attempt_id', -1);
if (dirty()) fail(8);

// Checking a field appends it; unchecking returns it to the unselected fields.
const exitCode = form.querySelector('input[value="exit_code"]');
exitCode.checked = true;
exitCode.dispatchEvent(new window.Event('change', { bubbles: true }));
if (JSON.stringify(saved()) !== JSON.stringify(['attempt_id', 'job_name', 'project', 'exit_code'])) fail(9, saved());
exitCode.checked = false;
exitCode.dispatchEvent(new window.Event('change', { bubbles: true }));
if (exitCode.closest('.notification-field-grid') === null || JSON.stringify(saved()) !== JSON.stringify(settings.fields)) fail(10, saved());

// The default order puts the project and job name before IDs.
const reset = form.querySelector('.notification-field-default-order');
if (reset.disabled) fail(11);
reset.click();
if (JSON.stringify(saved()) !== JSON.stringify(['project', 'job_name', 'attempt_id'])) fail(12, saved());
if (!reset.disabled || !dirty()) fail(13);
`
	output, err := exec.Command("node", "-e", script).CombinedOutput()
	if err != nil {
		t.Fatalf("notification field order check failed: %v\n%s", err, output)
	}
}
