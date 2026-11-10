package webui

import (
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"html"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rotari-dev/rotari/internal/diagnose"
	"github.com/rotari-dev/rotari/internal/joblist"
)

//go:embed assets/web_template.html
var webTemplateHTML string

//go:embed assets/web_app_core.js
var webAppCoreJS string

//go:embed assets/web_app_actions.js
var webAppActionsJS string

//go:embed assets/web_app_logs.js
var webAppLogsJS string

//go:embed assets/vendor/highlight.min.js
var highlightJS string

//go:embed assets/vendor/highlightjs-license.txt
var highlightLicense string

//go:embed assets/web_app_markdown.js
var webAppMarkdownJS string

//go:embed assets/web_app_highlight.js
var webAppHighlightJS string

//go:embed assets/web_app_highlight.css
var webAppHighlightCSS string

//go:embed assets/web_app_previews.js
var webAppPreviewsJS string

//go:embed assets/web_app_artifacts.js
var webAppArtifactsJS string

//go:embed assets/web_app_launches.js
var webAppLaunchesJS string

//go:embed assets/web_app_tables.js
var webAppTablesJS string

//go:embed assets/web_app_charts.js
var webAppChartsJS string

//go:embed assets/web_app_matrix.js
var webAppMatrixJS string

//go:embed assets/web_app_notifications.js
var webAppNotificationsJS string

//go:embed assets/web_app_search.js
var webAppSearchJS string

//go:embed assets/web_app_activity.js
var webAppActivityJS string

//go:embed assets/web_app_bootstrap.js
var webAppBootstrapJS string

//go:embed assets/web_static_bootstrap.js
var webStaticBootstrapJS string

//go:embed assets/jobs_template.html
var jobsTemplateHTML string

//go:embed assets/web_info_styles.css
var webInfoStylesCSS string

//go:embed assets/web_shell.js
var webShellJS string

//go:embed assets/web_tokens.css
var webTokensCSS string

//go:embed assets/web_styles.css
var webStylesCSS string

//go:embed assets/web_shell.css
var webShellCSS string

type webBaseDir struct {
	ID      string `json:"id"`
	Path    string `json:"path"`
	Current bool   `json:"current"`
	// Parent and Name label the basedir in the sidebar and breadcrumbs: its
	// parent directory tells it apart from other basedirs, which usually
	// share a name such as .rotari-state.
	Parent string `json:"parent"`
	Name   string `json:"name"`
}

// newWebBaseDir describes a registered basedir, with its label.
func newWebBaseDir(id, path string, current bool) webBaseDir {
	name := filepath.Base(path)
	parent := filepath.Base(filepath.Dir(path))
	if parent == string(filepath.Separator) || parent == "." || parent == name {
		parent = ""
	}
	return webBaseDir{ID: id, Path: path, Current: current, Parent: parent, Name: name}
}

// basedirLabelHTML is a basedir's label: its parent directory in bold and its
// own name. The JS sidebar builds the same markup from Parent and Name.
func basedirLabelHTML(entry webBaseDir) string {
	name := html.EscapeString(entry.Name)
	if entry.Parent == "" {
		return `<span class="basedir-name">` + name + `</span>`
	}
	return `<span class="basedir-parent">` + html.EscapeString(entry.Parent) + `</span><span class="basedir-name">/` + name + `</span>`
}

// basedirBellHTML is the switch that notifies about runs in one basedir. It
// is a checkbox drawn as a bell; action names the page's handler.
func basedirBellHTML(entry webBaseDir, extra, action string) string {
	return `<label class="basedir-bell" title="Enable notifications"><input class="basedir-notification-toggle" type="checkbox" data-basedir-id="` +
		html.EscapeString(entry.ID) + `"` + extra + ` aria-label="Notify about runs in this basedir" data-change-action="` + action + `" />` + bellIconSVG + `</label>`
}

// allBasedirsBellHTML is the switch beside the Registered basedirs heading
// that turns every basedir bell on or off at once. It stays hidden until the
// page's script finds more than one basedir bell.
func allBasedirsBellHTML(action string) string {
	return `<label class="basedir-bell basedir-bell-all" title="Enable notifications for all basedirs" hidden><input class="basedir-notification-all" type="checkbox" aria-label="Notify about runs in all basedirs" data-change-action="` + action + `" />` + bellIconSVG + `</label>`
}

const bellIconSVG = `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M6 16V11a6 6 0 1 1 12 0v5l1.5 2h-15z"></path><path d="M10 20a2 2 0 0 0 4 0"></path></svg>`

func composeWebHTML(executors []string, notifications bool, bootstrap string, basedirLists ...[]webBaseDir) string {
	return composeWebHTMLWithSession(executors, notifications, bootstrap, "", basedirLists...)
}

func composeWebHTMLWithSession(executors []string, notifications bool, bootstrap, notificationSession string, basedirLists ...[]webBaseDir) string {
	return composeWebPage(livePage{}, executors, notifications, bootstrap, notificationSession, basedirLists...)
}

// livePage is what only a page from the live server has. Its zero value is a
// static export's: no response headers to carry a policy, and nothing to connect to.
type livePage struct {
	// nonce is the one the page's scripts run under.
	nonce string
	// terminal offers the terminal panel.
	terminal bool
	// about is which rotari serves the page, at the foot of its Settings menu.
	about About
}

// aboutSection says, at the foot of the Settings menu, which rotari serves the page: its
// version, and its binary's path with a button to copy it, which web_shell.js gives a
// button to send it beside. A page with nothing to say has nothing there.
func (page livePage) aboutSection() string {
	if page.about.Version == "" {
		return ""
	}
	var builder strings.Builder
	builder.WriteString(`<div class="settings-about"><div class="settings-about-version">`)
	builder.WriteString(html.EscapeString(page.about.Version))
	builder.WriteString(`</div><div class="settings-about-path">`)
	if page.about.Executable != "" {
		builder.WriteString(`<code title="`)
		builder.WriteString(html.EscapeString(page.about.Executable))
		builder.WriteString(`"><bdi>`)
		builder.WriteString(html.EscapeString(page.about.Executable))
		builder.WriteString(`</bdi></code>`)
		writeJobsCopyButton(&builder, page.about.Executable, "path of this rotari")
	} else {
		builder.WriteString(`<span class="settings-about-unknown">Its path is not known: `)
		builder.WriteString(html.EscapeString(page.about.ExecutableError))
		builder.WriteString(`</span>`)
	}
	builder.WriteString(`</div></div>`)
	return builder.String()
}

// terminalControl is the top bar's button, whose presence tells web_shell.js the
// terminal is available.
func (page livePage) terminalControl() string {
	if !page.terminal {
		return ""
	}
	return `<button id="terminal-button" type="button" aria-pressed="false" data-action="toggleTerminalPanel">Terminal</button>`
}

// composeWebPage is the application page.
func composeWebPage(page livePage, executors []string, notifications bool, bootstrap, notificationSession string, basedirLists ...[]webBaseDir) string {
	executorJSON, _ := json.Marshal(executors)
	diagnosisGuidanceJSON, _ := json.Marshal(struct {
		NoMatchNext     string `json:"noMatchNext"`
		UnavailableNext string `json:"unavailableNext"`
		OutdatedNote    string `json:"outdatedNote"`
	}{diagnose.NoMatchNext, diagnose.UnavailableNext, diagnose.OutdatedNote})
	basedirs := []webBaseDir{}
	if len(basedirLists) > 0 {
		basedirs = basedirLists[0]
	}
	basedirJSON, _ := json.Marshal(basedirs)
	webAppJS := strings.Join([]string{
		"const diagnosisGuidance = " + string(diagnosisGuidanceJSON) + ";",
		webAppCoreJS, webAppActionsJS, webAppLogsJS, highlightJS, webAppHighlightJS, webAppPreviewsJS, webAppMarkdownJS, webAppArtifactsJS, webAppLaunchesJS, webAppTablesJS, webAppChartsJS, webAppMatrixJS, webAppNotificationsJS, webAppSearchJS, webAppActivityJS, webAppBootstrapJS,
	}, "\n")
	// The scripts go in first, since they hold placeholders of their own. A
	// Replacer does not search what it inserts, so a value that contains the
	// text of a placeholder stays as written.
	template := strings.NewReplacer("__ROTARI_SHELL_SCRIPT__", webShellJS, "__ROTARI_WEB_APP__", webAppJS).Replace(webTemplateHTML)
	return strings.NewReplacer(
		scriptNonceSlot, scriptNonceAttribute(page.nonce),
		"__ROTARI_TERMINAL_CONTROL__", page.terminalControl(),
		"__ROTARI_ABOUT__", page.aboutSection(),
		"__ROTARI_BASEDIRS__", string(basedirJSON),
		"__ROTARI_ALL_BASEDIRS_BELL__", allBasedirsBellHTML("toggleAllNotificationBasedirs"),
		"__ROTARI_EXECUTORS__", string(executorJSON),
		"__ROTARI_STATUS_TONES__", statusTonesJSON(),
		"__ROTARI_BRAND_ICON__", brandIcon(),
		"__ROTARI_NOTIFICATION_ICON__", faviconDataURL(webFaviconDarkSVG),
		"__ROTARI_NOTIFICATION_DEFAULT__", strconv.FormatBool(notifications),
		"__ROTARI_NOTIFICATION_SESSION__", notificationSession,
		"__ROTARI_STATIC_BOOTSTRAP__", bootstrap,
	).Replace(template)
}

func composeStaticBootstrap(state, logs, reports, configTargets, configs, wordClouds, artifacts, artifactData, activity string) string {
	launchers, _ := json.Marshal(staticLaunchAvailability())
	return strings.NewReplacer(
		"__ROTARI_STATIC_STATE_DATA__", state,
		"__ROTARI_STATIC_LOGS_DATA__", logs,
		"__ROTARI_STATIC_REPORTS_DATA__", reports,
		"__ROTARI_STATIC_CONFIG_TARGETS_DATA__", configTargets,
		"__ROTARI_STATIC_CONFIGS_DATA__", configs,
		"__ROTARI_STATIC_WORD_CLOUDS_DATA__", wordClouds,
		"__ROTARI_STATIC_ARTIFACTS_DATA__", artifacts,
		"__ROTARI_STATIC_ARTIFACT_CONTENTS_DATA__", artifactData,
		"__ROTARI_STATIC_ACTIVITY_DATA__", activity,
		"__ROTARI_STATIC_LAUNCHERS_DATA__", string(launchers),
	).Replace(webStaticBootstrapJS)
}

// composeInfoHTML fills template in one pass, with the page's own placeholders
// and values as further pairs, so a value keeps the text of any placeholder.
func composeInfoHTML(template, homePath, content string, values ...string) string {
	styles := fontFaceCSS(homePath+"fonts/") + "\n" + webTokensCSS + "\n" + webInfoStylesCSS + "\n" + webShellCSS
	searchPath := strings.TrimRight(homePath, "/") + "/search/"
	activityPath := strings.TrimRight(homePath, "/") + "/activity/"
	return strings.NewReplacer(append([]string{
		"__ROTARI_FAVICON_LINKS__", faviconLinks(),
		"__ROTARI_SHELL_SCRIPT__", webShellJS,
		"__ROTARI_INFO_STYLES__", styles,
		"__ROTARI_BRAND_ICON__", brandIcon(),
		"__ROTARI_HOME_PATH__", html.EscapeString(homePath),
		"__ROTARI_SEARCH_PATH__", html.EscapeString(searchPath),
		"__ROTARI_ACTIVITY_PATH__", html.EscapeString(activityPath),
		"__ROTARI_CONTENT__", content,
	}, values...)...).Replace(template)
}

func jobsHTML(homePath string, projects []string, rows []joblist.Row, selection jobsSelection, canFilter, notifications bool, basedirLists ...[]webBaseDir) string {
	return jobsHTMLWithSession(homePath, projects, rows, selection, canFilter, notifications, "", basedirLists...)
}

// jobsHTMLWithSession renders the Recent jobs page for selection; the zero
// selection is the default window. The live page's script draws the
// window presets and range bar into .jobs-range, with this basedir's runs
// from the Activity API behind the bar.
func jobsHTMLWithSession(homePath string, projects []string, rows []joblist.Row, selection jobsSelection, canFilter, notifications bool, notificationSession string, basedirLists ...[]webBaseDir) string {
	return jobsPage(livePage{}, homePath, projects, rows, selection, canFilter, notifications, notificationSession, basedirLists...)
}

// jobsPage is the Recent jobs page.
func jobsPage(page livePage, homePath string, projects []string, rows []joblist.Row, selection jobsSelection, canFilter, notifications bool, notificationSession string, basedirLists ...[]webBaseDir) string {
	var builder strings.Builder
	if canFilter {
		basedirID := ""
		if len(basedirLists) > 0 {
			basedirID = jobsActiveBasedirID(homePath, basedirLists[0])
		}
		window := ""
		for _, preset := range jobsWindows {
			if selection.From.IsZero() && (preset.Window == selection.Window || selection.Window == 0 && preset.Window == joblist.DefaultSince) {
				window = preset.Text
			}
		}
		builder.WriteString(`<div class="jobs-range" data-activity-url="`)
		builder.WriteString(html.EscapeString(homePath + "api/activity"))
		builder.WriteString(`" data-basedir-id="`)
		builder.WriteString(html.EscapeString(basedirID))
		builder.WriteString(`" data-window="`)
		builder.WriteString(window)
		if !selection.From.IsZero() {
			builder.WriteString(`" data-from="`)
			builder.WriteString(selection.From.UTC().Format(time.RFC3339))
			builder.WriteString(`" data-to="`)
			builder.WriteString(selection.To.UTC().Format(time.RFC3339))
		}
		builder.WriteString(`"></div>`)
	}
	var sidebar strings.Builder
	if len(basedirLists) > 0 {
		writeJobsBasedirSidebar(&sidebar, homePath, projects, basedirLists[0])
	} else {
		writeJobsSidebarProjects(&sidebar, homePath, projects)
	}
	scrollKey := ""
	if len(basedirLists) > 0 {
		scrollKey = currentBasedirScrollID(basedirLists[0])
	}
	notificationControl := `<button id="notify-toggle" type="button" data-action="toggleJobsNotifications">Notification off</button>`
	var toolbar string
	if canFilter {
		toolbar = `<button id="refresh-button" type="button" data-action="reloadPage">Refresh</button>`
	}
	var crumbs strings.Builder
	if len(basedirLists) > 0 {
		activeID := jobsActiveBasedirID(homePath, basedirLists[0])
		for _, entry := range basedirLists[0] {
			if entry.ID == activeID {
				writeBasedirCrumb(&crumbs, homePath, entry)
				crumbs.WriteString(`<span class="crumb-sep">/</span>`)
			}
		}
	}
	crumbs.WriteString(`<span class="crumb-here">Recent jobs</span>`)
	values := []string{
		scriptNonceSlot, scriptNonceAttribute(page.nonce),
		"__ROTARI_TERMINAL_CONTROL__", page.terminalControl(),
		"__ROTARI_ABOUT__", page.aboutSection(),
		"__ROTARI_JOBS_PROJECTS__", sidebar.String(),
		"__ROTARI_ALL_BASEDIRS_BELL__", allBasedirsBellHTML("toggleAllJobsNotificationBasedirs"),
		"__ROTARI_BASEDIR_SCROLL_KEY__", scrollKey,
		"__ROTARI_JOBS_CRUMBS__", crumbs.String(),
		"__ROTARI_JOBS_SCOPE__", html.EscapeString(jobsScopeText(selection)),
		"__ROTARI_JOBS_NOTIFICATION_CONTROL__", notificationControl,
		"__ROTARI_JOBS_TOOLBAR__", toolbar,
		"__ROTARI_JOBS_LIVE__", strconv.FormatBool(canFilter),
		"__ROTARI_NOTIFICATION_DEFAULT__", strconv.FormatBool(notifications),
		"__ROTARI_NOTIFICATION_SESSION__", notificationSession,
		"__ROTARI_NOTIFICATION_ICON__", faviconDataURL(webFaviconDarkSVG),
	}
	if len(rows) == 0 {
		builder.WriteString(`<p class="meta">No unfinished or recently finished jobs found.</p>`)
		return composeInfoHTML(jobsTemplateHTML, homePath, builder.String(), values...)
	}
	builder.WriteString(`<section class="jobs-section"><table class="jobs-table"><thead><tr><th data-sort="state">State</th><th data-sort="project">Project</th><th data-sort="job">Job</th><th data-sort="attempt">Attempt</th><th data-sort="started">Started</th><th data-sort="finished">Finished</th><th data-sort="elapsed">Elapsed</th></tr></thead><tbody>`)
	for _, row := range rows {
		builder.WriteString(`<tr><td class="jobs-state">`)
		builder.WriteString(statusPillHTML(row.State))
		builder.WriteString(`</td><td><a href="`)
		builder.WriteString(html.EscapeString(homePath))
		builder.WriteString(`project/`)
		builder.WriteString(url.PathEscape(row.Project))
		builder.WriteString(`">`)
		builder.WriteString(html.EscapeString(row.Project))
		builder.WriteString(`</a>`)
		builder.WriteString(`</td><td><a href="`)
		builder.WriteString(html.EscapeString(homePath))
		builder.WriteString(`project/`)
		builder.WriteString(url.PathEscape(row.Project))
		builder.WriteString(`/run/`)
		builder.WriteString(url.PathEscape(row.RunID))
		builder.WriteString(`">`)
		builder.WriteString(html.EscapeString(row.JobName))
		// The command is shown under the job name, as on the run page.
		builder.WriteString(`</a><div class="jobs-command"><code title="`)
		builder.WriteString(html.EscapeString(row.FullCommand))
		builder.WriteString(`">`)
		builder.WriteString(html.EscapeString(row.Command))
		builder.WriteString(`</code>`)
		writeJobsCopyButton(&builder, row.FullCommand, "command")
		builder.WriteString(`</div>`)
		if row.AttemptID == "" {
			builder.WriteString(`</td><td>-`)
		} else {
			builder.WriteString(`</td><td class="jobs-attempt" data-sort-value="`)
			builder.WriteString(html.EscapeString(row.AttemptID))
			builder.WriteString(`">`)
			builder.WriteString(idTailHTML(row.AttemptID))
			writeJobsCopyButton(&builder, row.AttemptID, "attempt ID")
		}
		builder.WriteString(`</td><td class="jobs-time" data-sort-value="`)
		if !row.StartedAt.IsZero() {
			builder.WriteString(html.EscapeString(row.StartedAt.Format(time.RFC3339Nano)))
		}
		builder.WriteString(`">`)
		if row.StartedAt.IsZero() {
			builder.WriteString(`-`)
		} else {
			builder.WriteString(html.EscapeString(joblist.FormatTimestamp(row.StartedAt)))
		}
		builder.WriteString(`</td><td class="jobs-time" data-sort-value="`)
		if !row.FinishedAt.IsZero() {
			builder.WriteString(html.EscapeString(row.FinishedAt.Format(time.RFC3339Nano)))
		}
		builder.WriteString(`">`)
		if row.FinishedAt.IsZero() {
			builder.WriteString(`-`)
		} else {
			builder.WriteString(html.EscapeString(joblist.FormatTimestamp(row.FinishedAt)))
		}
		builder.WriteString(`</td><td>`)
		builder.WriteString(html.EscapeString(joblist.FormatRunTime(row.RunTime)))
		builder.WriteString(`</td></tr>`)
	}
	builder.WriteString(`</tbody></table></section>`)
	return composeInfoHTML(jobsTemplateHTML, homePath, builder.String(), values...)
}

func currentBasedirScrollID(basedirs []webBaseDir) string {
	for _, entry := range basedirs {
		if entry.Current {
			return entry.ID
		}
	}
	return ""
}

func writeJobsBasedirSidebar(builder *strings.Builder, homePath string, projects []string, basedirs []webBaseDir) {
	if len(basedirs) == 0 {
		writeJobsSidebarProjects(builder, homePath, projects)
		return
	}
	activeID := jobsActiveBasedirID(homePath, basedirs)
	for _, entry := range basedirs {
		writeJobsBasedirEntry(builder, homePath, projects, entry, activeID)
	}
}

func jobsActiveBasedirID(homePath string, basedirs []webBaseDir) string {
	if strings.HasPrefix(homePath, basedirRoutePrefix) {
		return strings.Split(strings.TrimPrefix(homePath, basedirRoutePrefix), "/")[0]
	}
	for _, entry := range basedirs {
		if entry.Current {
			return entry.ID
		}
	}
	return ""
}

func writeJobsBasedirEntry(builder *strings.Builder, homePath string, currentProjects []string, entry webBaseDir, activeID string) {
	active := entry.ID == activeID
	basePath := jobsBasedirHomePath(homePath, entry, active)
	projectNames := jobsBasedirProjects(entry, active, currentProjects)
	builder.WriteString(`<div class="sidebar-project basedir-entry`)
	if active {
		builder.WriteString(` expanded`)
	}
	builder.WriteString(`" data-basedir-id="`)
	builder.WriteString(html.EscapeString(entry.ID))
	builder.WriteString(`"><div class="sidebar-project-row basedir-row"><button type="button" class="sidebar-toggle" aria-expanded="`)
	builder.WriteString(strconv.FormatBool(active))
	builder.WriteString(`" aria-label="Toggle projects" data-action="toggleJobsSidebar"></button><a class="sidebar-project-link basedir-path" title="`)
	builder.WriteString(html.EscapeString(entry.Path))
	builder.WriteString(`" href="`)
	builder.WriteString(html.EscapeString(basePath))
	builder.WriteString(`">`)
	builder.WriteString(basedirLabelHTML(entry))
	builder.WriteString(`</a>`)
	builder.WriteString(basedirBellHTML(entry, "", "toggleJobsNotificationBasedir"))
	builder.WriteString(`</div><div class="sidebar-projects basedir-contents"`)
	if !active {
		builder.WriteString(` hidden`)
	}
	builder.WriteString(`><div class="sidebar-project expanded all-projects"><div class="sidebar-project-row"><button type="button" class="sidebar-toggle" aria-expanded="true" aria-label="Toggle project list" data-action="toggleJobsSidebar"></button><a class="sidebar-project-link" href="`)
	builder.WriteString(html.EscapeString(basePath))
	builder.WriteString(`">All projects</a></div><div class="sidebar-projects project-list">`)
	for _, project := range projectNames {
		writeJobsBasedirProject(builder, basePath, project)
	}
	builder.WriteString(`</div></div><a class="sidebar-run`)
	if active {
		builder.WriteString(` active`)
	}
	builder.WriteString(`" href="`)
	builder.WriteString(html.EscapeString(basePath))
	builder.WriteString(`jobs/">Recent jobs</a></div></div>`)
}

// writeBasedirCrumb writes the breadcrumb that names a page's basedir, with
// a button that copies its full path.
func writeBasedirCrumb(builder *strings.Builder, homePath string, entry webBaseDir) {
	builder.WriteString(`<a class="crumb-basedir" href="`)
	builder.WriteString(html.EscapeString(homePath))
	builder.WriteString(`" title="`)
	builder.WriteString(html.EscapeString(entry.Path))
	builder.WriteString(`">`)
	builder.WriteString(basedirLabelHTML(entry))
	builder.WriteString(`</a>`)
	writeJobsCopyButton(builder, entry.Path, "state directory path")
}

func jobsBasedirHomePath(homePath string, entry webBaseDir, active bool) string {
	if active {
		return homePath
	}
	if entry.Current {
		return "/"
	}
	return basedirRoutePrefix + entry.ID + "/"
}

func jobsBasedirProjects(entry webBaseDir, active bool, currentProjects []string) []string {
	if active {
		return currentProjects
	}
	projects, err := joblist.Projects(entry.Path, "")
	if err != nil {
		return nil
	}
	return projects
}

func writeJobsBasedirProject(builder *strings.Builder, basePath, project string) {
	builder.WriteString(`<div class="sidebar-project-row"><span class="sidebar-toggle-placeholder" aria-hidden="true"></span><a class="sidebar-project-link" href="`)
	builder.WriteString(html.EscapeString(basePath))
	builder.WriteString(`project/`)
	builder.WriteString(url.PathEscape(project))
	builder.WriteString(`">`)
	builder.WriteString(html.EscapeString(project))
	builder.WriteString(`</a></div>`)
}

func writeJobsSidebarProjects(builder *strings.Builder, homePath string, projects []string) {
	builder.WriteString(`<div class="sidebar-project expanded" id="sidebar-root"><div class="sidebar-project-row"><button type="button" class="sidebar-toggle" aria-expanded="true" aria-label="Toggle all projects" data-action="toggleJobsSidebar"></button><a class="sidebar-project-link" href="`)
	builder.WriteString(html.EscapeString(homePath))
	builder.WriteString(`">All projects</a></div><div class="sidebar-projects" id="sidebar-projects">`)
	for _, project := range projects {
		builder.WriteString(`<div class="sidebar-project-row"><span class="sidebar-toggle-placeholder" aria-hidden="true"></span><a class="sidebar-project-link" href="`)
		builder.WriteString(html.EscapeString(homePath))
		builder.WriteString(`project/`)
		builder.WriteString(url.PathEscape(project))
		builder.WriteString(`">`)
		builder.WriteString(html.EscapeString(project))
		builder.WriteString(`</a></div>`)
	}
	builder.WriteString(`</div></div>`)
}

// idTailHTML shows a long ID by its end, which tells attempts apart, with a
// leading ellipsis (.id-tail in web_shell.css); the text and title keep the
// whole ID. web_app_core.js writes the same markup.
func idTailHTML(id string) string {
	escaped := html.EscapeString(id)
	return `<span class="id-tail" title="` + escaped + `"><bdi>` + escaped + `</bdi></span>`
}

func writeJobsCopyButton(builder *strings.Builder, value, label string) {
	builder.WriteString(`<button class="jobs-copy" type="button" title="Copy `)
	builder.WriteString(html.EscapeString(label))
	builder.WriteString(`" aria-label="Copy `)
	builder.WriteString(html.EscapeString(label))
	builder.WriteString(`" data-copy-value="`)
	builder.WriteString(html.EscapeString(value))
	builder.WriteString(`" data-action="copyJobsValue"><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M9 9H5a1 1 0 0 0-1 1v9a1 1 0 0 0 1 1h9a1 1 0 0 0 1-1v-4"></path><rect x="9" y="4" width="11" height="11" rx="1"></rect></svg></button>`)
}

//go:embed assets/favicon-dark.svg
var webFaviconDarkSVG []byte

//go:embed assets/favicon-light.svg
var webFaviconLightSVG []byte

func faviconDataURL(svg []byte) string {
	return "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString(svg)
}

func faviconLinks() string {
	return `<link rel="icon" type="image/svg+xml" media="(prefers-color-scheme: dark)" href="` + faviconDataURL(webFaviconDarkSVG) + `"><link rel="icon" type="image/svg+xml" media="(prefers-color-scheme: light)" href="` + faviconDataURL(webFaviconLightSVG) + `">`
}

// brandIcon renders the favicon artwork inline; the web UI always uses the dark theme, so only that variant is needed.
// brandIcon embeds both favicons; web_tokens.css shows the one that matches
// the page's theme.
func brandIcon() string {
	return `<img class="brand-icon" alt="" src="` + faviconDataURL(webFaviconLightSVG) + `" data-theme-icon="light">` +
		`<img class="brand-icon" alt="" src="` + faviconDataURL(webFaviconDarkSVG) + `" data-theme-icon="dark">`
}
