package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rotari-dev/rotari/internal/diagnose"
	"github.com/rotari-dev/rotari/internal/model"
	"github.com/rotari-dev/rotari/internal/notification"
	"github.com/rotari-dev/rotari/internal/notificationlog"
	"github.com/rotari-dev/rotari/internal/state"
)

const webhookBatchWindow = 10 * time.Second

// pendingNotificationBatch is one run's events waiting for the batch window.
// settingsErr is why the run's webhook settings could not be loaded; the
// batch is still recorded for the inbox.
type pendingNotificationBatch struct {
	baseDir     string
	settings    notification.WebhookSettings
	settingsErr error
	events      []notification.Event
	timer       *time.Timer
}

// runNotificationManager batches a run's notification events and delivers
// each batch to the inbox record and, when configured, the webhook.
type runNotificationManager struct {
	mu      sync.Mutex
	pending map[string]*pendingNotificationBatch
	after   func(time.Duration, func()) *time.Timer
	now     func() time.Time
	send    func(notification.WebhookSettings, string, notification.Batch)
	record  func(baseDir string, events []notification.Event) error
}

// newRunNotificationManager records batches under masterDir, or under the
// resolved master directory when masterDir is empty.
func newRunNotificationManager(masterDir string) *runNotificationManager {
	manager := &runNotificationManager{
		pending: make(map[string]*pendingNotificationBatch),
		after:   time.AfterFunc,
		now:     time.Now,
	}
	manager.send = sendWebhookBatch
	manager.record = func(baseDir string, events []notification.Event) error {
		log := notificationlog.Open(masterDir)
		if masterDir == "" {
			var err error
			if log, err = notificationlog.Default(); err != nil {
				return err
			}
		}
		return log.Append(baseDir, events)
	}
	return manager
}

var runNotifications = newRunNotificationManager("")

func (manager *runNotificationManager) JobFinished(paths state.ProjectPaths, runID, runName string, job model.JobSpec, result model.JobResult) {
	event := notification.NewJobEvent(paths.ProjectName, runID, runName, job, result, manager.now())
	event.SetDiagnosisOutdated(diagnose.Outdated(result))
	if runDir, err := state.SafeJoin(paths.RunsDir, runID); err == nil {
		event.SetTimestamps(state.ReadJobTimestamp(runDir, job.ID, "submitted_at"), state.ReadJobTimestamp(runDir, job.ID, "finished_at"))
	}
	manager.add(paths, runID, event, false)
}

func (manager *runNotificationManager) RunFinished(paths state.ProjectPaths, runID string, _ int) {
	runDir, err := state.SafeJoin(paths.RunsDir, runID)
	if err != nil {
		printErrorf("WARNING: cannot prepare run notification: %v", err)
		return
	}
	summary, err := state.LoadRunSummary(filepath.Join(runDir, "summary.json"))
	if err != nil {
		printErrorf("WARNING: cannot load run summary for notifications: %v", err)
		return
	}
	manager.add(paths, runID, notification.NewRunEvent(paths.ProjectName, summary, manager.now()), true)
}

func (manager *runNotificationManager) add(paths state.ProjectPaths, runID string, event notification.Event, flush bool) {
	key := paths.BaseDir + "\x00" + paths.ProjectName + "\x00" + runID
	manager.mu.Lock()
	pending := manager.pending[key]
	if pending == nil {
		baseDir, err := filepath.Abs(paths.BaseDir)
		if err != nil {
			manager.mu.Unlock()
			printErrorf("WARNING: cannot resolve basedir for notifications: %v", err)
			return
		}
		pending = &pendingNotificationBatch{baseDir: baseDir}
		pending.settings, pending.settingsErr = loadRunWebhookSettings(paths, runID)
		if pending.settingsErr != nil {
			printErrorf("WARNING: cannot load webhook settings: %v", pending.settingsErr)
		}
		manager.pending[key] = pending
		pending.timer = manager.after(webhookBatchWindow, func() { manager.flush(key) })
	}
	pending.events = append(pending.events, event)
	if !flush {
		manager.mu.Unlock()
		return
	}
	if pending.timer != nil {
		pending.timer.Stop()
	}
	delete(manager.pending, key)
	manager.mu.Unlock()
	manager.deliver(pending)
}

func (manager *runNotificationManager) flush(key string) {
	manager.mu.Lock()
	pending := manager.pending[key]
	delete(manager.pending, key)
	manager.mu.Unlock()
	if pending != nil {
		manager.deliver(pending)
	}
}

func (manager *runNotificationManager) deliver(pending *pendingNotificationBatch) {
	if err := manager.record(pending.baseDir, pending.events); err != nil {
		printErrorf("WARNING: cannot record notification for the Web inbox: %v", err)
	}
	if pending.settingsErr != nil {
		return
	}
	batch := notification.NewBatch(pending.events, pending.settings.ChannelSettings, manager.now())
	if pending.settings.URL != "" && !batch.Empty() {
		manager.send(pending.settings, pending.baseDir, batch)
	}
}

func loadRunWebhookSettings(paths state.ProjectPaths, runID string) (notification.WebhookSettings, error) {
	loaded, err := loadRunNotificationSettings(paths, runID)
	if err != nil {
		return notification.WebhookSettings{}, err
	}
	settings := loaded.Webhook
	if value := strings.TrimSpace(os.Getenv(envWebhookURL)); value != "" {
		settings.URL = value
	}
	settings.URL = strings.TrimSpace(settings.URL)
	return settings, nil
}

func loadRunNotificationSettings(paths state.ProjectPaths, runID string) (notification.Settings, error) {
	runDir, err := state.SafeJoin(paths.RunsDir, runID)
	if err != nil {
		return notification.Settings{}, err
	}
	context, err := state.LoadContext(jsonStore(), runDir)
	if err == nil {
		for _, fileName := range context.ConfigSnapshotFiles {
			if fileName != notification.FileName {
				continue
			}
			data, err := os.ReadFile(filepath.Join(runDir, "configs", fileName))
			if err != nil {
				return notification.Settings{}, err
			}
			return notification.Parse(data)
		}
	}
	loaded, err := notification.Load(paths.BaseDir, paths.ProjectName)
	return loaded.Settings, err
}

func sendWebhookBatch(settings notification.WebhookSettings, baseDir string, batch notification.Batch) {
	parsed, err := url.Parse(settings.URL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		printErrorf("WARNING: invalid %s URL", envWebhookURL)
		return
	}
	payload := notification.NewPayload(batch, settings.Fields, settings.WebURL, baseDir)
	encoder, ok := webhookEncoders[normalizeWebhookFormat(settings.Format)]
	if !ok {
		printErrorf("WARNING: unsupported webhook format %q", settings.Format)
		return
	}
	encodedPayload, err := encoder.Encode(payload)
	if err != nil {
		printErrorf("WARNING: cannot prepare webhook notification: %v", err)
		return
	}
	data, err := json.Marshal(encodedPayload)
	if err != nil {
		printErrorf("WARNING: cannot encode webhook notification: %v", err)
		return
	}
	request, err := http.NewRequest(http.MethodPost, settings.URL, bytes.NewReader(data))
	if err != nil {
		printErrorf("WARNING: cannot create webhook notification: %v", err)
		return
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := (&http.Client{Timeout: 10 * time.Second}).Do(request)
	if err != nil {
		printErrorf("WARNING: webhook notification failed: %v", err)
		return
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		printErrorf("WARNING: webhook notification returned HTTP %d", response.StatusCode)
	}
}
