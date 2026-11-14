package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/rotari-dev/rotari/internal/basedirregistry"
	"github.com/rotari-dev/rotari/internal/notification"
	"github.com/rotari-dev/rotari/internal/notificationlog"
	"github.com/rotari-dev/rotari/internal/runwatch"
	stateinternal "github.com/rotari-dev/rotari/internal/state"
	"github.com/rotari-dev/rotari/internal/webdetach"
	"github.com/rotari-dev/rotari/internal/weblog"
	"github.com/rotari-dev/rotari/internal/webui"
)

// cmdWeb serves the embedded Web UI or writes a static export of project state.
func cmdWeb(args []string) int {
	flags, code := parseWebFlags(args)
	if code != 0 {
		return code
	}
	if flags.shutdown {
		return shutdownDetachedWeb(flags.yes)
	}
	baseDir, _, err := stateinternal.ResolveBaseDir(flags.basedir)
	if err != nil {
		printErrorf("failed to resolve state directory: %v", err)
		return 1
	}
	options := webOptions(baseDir, flags.allowControl, flags.notifications)
	options.About = webAbout()
	options.LauncherFile, _ = webLauncherFile()
	// An invalid notifications.toml is reported before serving; each
	// project's file is read again whenever the inbox renders its records.
	if _, err := notification.Load(baseDir, ""); err != nil {
		printErrorf("failed to load notification configuration: %v", err)
		return 1
	}
	if flags.staticDir != "" {
		options.StaticArtifactContents = flags.staticArtifactContents
		options.StaticArtifactsCopied = func(files int, bytes int64) {
			if files == 0 {
				return
			}
			printWarningf("Copied %d artifact files (%d bytes) into %s; anyone who can read the export can read them.", files, bytes, flags.staticDir)
		}
		return generateStaticWeb(flags.staticDir, options)
	}
	mode := takeWebChildMode()
	switch {
	case mode == webChildDetached:
		child, err := holdWebDetached()
		if errors.Is(err, webdetach.ErrRunning) {
			printErrorf("%v; run rotari web --detach again to see it", err)
			return 1
		}
		if err != nil {
			printErrorf("cannot detach rotari web: %v", err)
			return 1
		}
		defer child.close()
		flags.detached = child
	case strings.HasPrefix(mode, webChildAttachedPrefix):
		child, err := attachWebChild(strings.TrimPrefix(mode, webChildAttachedPrefix))
		if err != nil {
			printErrorf("cannot start rotari web: %v", err)
			return 1
		}
		defer child.close()
		flags.detached = child
	case mode != "":
		printErrorf("cannot start rotari web: %s=%q is not one rotari sets", webDetachedChildEnv, mode)
		return 1
	case flags.detach:
		return startDetachedWeb(flags, baseDir)
	case webRunsAsClient(flags):
		return runWebClient(flags)
	}
	return serveWeb(flags, baseDir, options)
}

type webCommandFlags struct {
	basedir, host, staticDir, authToken string
	port                                int
	allowControl, notifications         bool
	authGenerated                       bool
	portExplicit                        bool
	// allowExec enables launching registered commands; allowedHosts are
	// extra Host names accepted while it is on.
	allowExec    bool
	allowedHosts []string
	// allowTerminal enables the Web UI's shells, as terminal says.
	allowTerminal bool
	terminal      webTerminalSettings
	// artifactRoots are absolute --artifact-root directories.
	artifactRoots []string
	// staticArtifactContents copies artifact files into a static export.
	staticArtifactContents bool
	// detach starts the server in the background; detached is set in that
	// background process.
	detach   bool
	detached *webDetachedChild
	// disconnectAction is what a closed terminal does to an attached server: stop or
	// detach.
	disconnectAction string
	// shutdown stops this host's detached server instead of serving; yes does so
	// without asking.
	shutdown, yes bool
}

func parseWebFlags(args []string) (webCommandFlags, int) {
	fs := newFlagSet("web")
	basedir := cliString(fs, "basedir", "")
	host := cliString(fs, "host", "127.0.0.1")
	port := cliInt(fs, "port", webui.DefaultPort)
	staticDir := cliString(fs, "static-dir", "")
	allowControl := cliBool(fs, "allow-control", true)
	authToken := cliString(fs, "auth-token", "")
	notifications := cliBool(fs, "notifications", true)
	var artifactRoots stringSliceFlag
	cliValue(fs, &artifactRoots, "artifact-root")
	staticArtifactContents := cliBool(fs, "static-artifact-contents", false)
	detach := cliBool(fs, "detach", false)
	disconnectAction := fs.String("disconnect-action", webDisconnectActionDefault(fs), webFlagUsage("disconnect-action"))
	shutdown := cliBool(fs, "shutdown", false)
	yes := cliBool(fs, "yes", false)
	// The terminal and launch options read the environment and config
	// strictly, and webTrustedOptions only trusted configs.
	configured := readWebConfigured(fs)
	allowExec := fs.Bool("allow-exec", configured.allowExec, webFlagUsage("allow-exec"))
	allowTerminal := fs.Bool("allow-terminal", configured.allowTerminal, webFlagUsage("allow-terminal"))
	terminalIdleTimeout := fs.Duration("terminal-idle-timeout", configured.idleLimit, webFlagUsage("terminal-idle-timeout"))
	terminalMaxShells := fs.Int("terminal-max-shells", configured.shellLimit, webFlagUsage("terminal-max-shells"))
	terminalMaxPerProject := fs.Int("terminal-max-shells-per-project", configured.groupLimit, webFlagUsage("terminal-max-shells-per-project"))
	terminalReplaySize := fs.String("terminal-replay-size", configured.replaySize, webFlagUsage("terminal-replay-size"))
	terminalShell := fs.String("terminal-shell", configured.shell, webFlagUsage("terminal-shell"))
	terminalIntegration := fs.Bool("terminal-shell-integration", configured.integration, webFlagUsage("terminal-shell-integration"))
	allowedHosts := stringSliceFlag(configured.allowedHosts)
	fs.Var(&allowedHosts, "allowed-host", webFlagUsage("allowed-host"))
	if err := cliParse(fs, args); err != nil {
		return webCommandFlags{}, 1
	}
	if *shutdown || *yes {
		return parseWebShutdownFlags(fs, *shutdown, *yes)
	}
	for _, name := range webTrustedOptions {
		warnIgnoredWebConfig(fs, name, configured.ignored[name], false)
	}
	if *allowExec || *allowTerminal {
		*host = trustedWebExposedValue(fs, "host", *host, "127.0.0.1")
		*authToken = trustedWebExposedValue(fs, "auth-token", *authToken, "")
	}
	if len(fs.Args()) != 0 || *port < 0 || *port > 65535 {
		printError("usage: " + cliUsage("web"))
		return webCommandFlags{}, 1
	}
	if *staticDir != "" {
		serverOptions := webStaticServerOptions(fs)
		if len(serverOptions) > 0 {
			printErrorf("--%s cannot be combined with --static-dir; these options only apply to the live web server", strings.Join(serverOptions, ", --"))
			return webCommandFlags{}, 1
		}
	}
	if *staticArtifactContents && *staticDir == "" {
		printError("--static-artifact-contents requires --static-dir")
		return webCommandFlags{}, 1
	}
	if *allowExec && !*allowControl {
		printError("--allow-exec requires job control; remove --allow-control=false")
		return webCommandFlags{}, 1
	}
	if *allowTerminal && !*allowControl {
		printError("--allow-terminal requires job control; remove --allow-control=false")
		return webCommandFlags{}, 1
	}
	for _, name := range webTerminalOptions {
		if cliOptionSet(fs, name) && !*allowTerminal {
			printErrorf("--%s only applies with --allow-terminal", name)
			return webCommandFlags{}, 1
		}
	}
	var terminalSettings webTerminalSettings
	if *allowTerminal {
		for _, name := range webTerminalOptions {
			if cliOptionSet(fs, name) {
				delete(configured.from, name)
			}
		}
		var err error
		terminalSettings, err = newWebTerminalSettings(*terminalIdleTimeout, *terminalMaxShells, *terminalMaxPerProject, *terminalReplaySize, *terminalShell, configured.from)
		if err != nil {
			printError(err)
			return webCommandFlags{}, 1
		}
		terminalSettings.integration = *terminalIntegration
	}
	if cliOptionSet(fs, "allowed-host") && !*allowExec && !*allowTerminal {
		printError("--allowed-host only applies with --allow-exec or --allow-terminal")
		return webCommandFlags{}, 1
	}
	for _, name := range allowedHosts {
		if !validAllowedHost(name) {
			printErrorf("--allowed-host %q must be a host name or IP address without a port", name)
			return webCommandFlags{}, 1
		}
	}
	if *disconnectAction != webDisconnectStop && *disconnectAction != webDisconnectDetach {
		printErrorf("--disconnect-action %q must be stop or detach", *disconnectAction)
		return webCommandFlags{}, 1
	}
	if *detach && cliOptionSet(fs, "disconnect-action") {
		printError("--disconnect-action only applies without --detach, which detaches at once")
		return webCommandFlags{}, 1
	}
	flags := webCommandFlags{basedir: *basedir, host: *host, port: *port, staticDir: *staticDir, authToken: *authToken, allowControl: *allowControl, notifications: *notifications, staticArtifactContents: *staticArtifactContents, detach: *detach, disconnectAction: *disconnectAction,
		allowExec: *allowExec, allowedHosts: allowedHosts, allowTerminal: *allowTerminal, terminal: terminalSettings}
	for _, root := range artifactRoots {
		absolute, err := filepath.Abs(root)
		if info, statErr := os.Stat(absolute); err != nil || statErr != nil || !info.IsDir() {
			printErrorf("--artifact-root %s is not a directory", root)
			return webCommandFlags{}, 1
		}
		flags.artifactRoots = append(flags.artifactRoots, absolute)
	}
	fs.Visit(func(flag *flag.Flag) {
		flags.portExplicit = flags.portExplicit || flag.Name == "port"
	})
	return flags, 0
}

func generateStaticWeb(output string, options webui.Options) int {
	if err := webui.GenerateStatic(output, options); err != nil {
		printErrorf("failed to generate static web: %v", err)
		return 1
	}
	return 0
}

// watchInterruptedRuns records the interrupted runs of baseDir and the
// registered basedirs for the Web inbox now and every runwatch.Interval
// while rotari web serves. A failure is warned about when it first appears
// or changes, not on every pass.
func watchInterruptedRuns(log notificationlog.Log, baseDir string, registered func() ([]string, error)) {
	lastProblem := ""
	for {
		baseDirs, err := registered()
		if err == nil {
			var current string
			if current, err = filepath.Abs(baseDir); err == nil && !slices.Contains(baseDirs, current) {
				baseDirs = append(baseDirs, current)
			}
		}
		if err == nil {
			_, err = runwatch.ReportInterrupted(log, baseDirs, time.Now())
		}
		problem := ""
		if err != nil {
			problem = err.Error()
		}
		if problem != "" && problem != lastProblem {
			printErrorf("WARNING: cannot record interrupted runs for the Web inbox: %v", err)
		}
		lastProblem = problem
		time.Sleep(runwatch.Interval)
	}
}

func serveWeb(flags webCommandFlags, baseDir string, options webui.Options) int {
	registry, err := basedirregistry.Default()
	if err != nil {
		printErrorf("failed to resolve base directory registry: %v", err)
		return 1
	}
	options.BaseDirs, err = registry.BaseDirs()
	if err != nil {
		printErrorf("failed to list registered base directories: %v", err)
		return 1
	}
	options.ReloadBaseDirs = registry.BaseDirs
	notificationLog, err := notificationlog.Default()
	if err != nil {
		printErrorf("failed to resolve notification history: %v", err)
		return 1
	}
	options.NotificationLog = &notificationLog
	go watchInterruptedRuns(notificationLog, baseDir, registry.BaseDirs)
	options.ArtifactRoots = flags.artifactRoots
	if flags.allowExec {
		launches, err := newWebLaunches()
		if err != nil {
			printErrorf("cannot enable launching: %v", err)
			return 1
		}
		defer launches.Manager.Close()
		options.Launches = launches
	}
	if flags.allowTerminal {
		keeper, err := newWebTerminal(flags.terminal)
		if err != nil {
			printErrorf("cannot enable the terminal: %v", err)
			return 1
		}
		defer keeper.Close()
		options.Terminal = keeper
	}
	if flags.allowExec || flags.allowTerminal {
		if flags.authToken == "" {
			var err error
			if flags.authToken, err = newWebAuthToken(); err != nil {
				printErrorf("cannot generate a Web token: %v", err)
				return 1
			}
			flags.authGenerated = true
		}
		printWebSignIn(flags, flags.authToken, flags.authGenerated)
	}
	if options.Launches != nil {
		printLaunchRegistry(options.Launches.Registry)
	}
	warnUnauthenticatedHost(flags, flags.authToken)
	if flags.authToken != "" {
		options.AuthEnabled = true
	}
	masterDir, err := stateinternal.ResolveMasterDir("")
	if err != nil {
		printErrorf("WARNING: cannot record Web events: %v", err)
	}
	var webLog *weblog.Recorder
	if err == nil {
		webLog = weblog.New(masterDir, "web", os.Stderr)
		defer webLog.Close()
	}
	// One shutdown, however it is asked for: SIGINT, SIGTERM, or the Web UI.
	shutdownRequested := make(chan struct{})
	var shutdownOnce sync.Once
	options.Shutdown = func() { shutdownOnce.Do(func() { close(shutdownRequested) }) }
	handler := webui.Handler(options)
	if flags.authToken != "" {
		var onSignOut []func()
		if options.Terminal != nil {
			onSignOut = append(onSignOut, options.Terminal.EndAll)
		}
		handler = webui.WithAuthToken(handler, flags.authToken, webui.AuthOptions{Recorder: webLog, OnSignOut: onSignOut})
	}
	if flags.allowExec || flags.allowTerminal {
		handler = webui.WithExecGuard(handler, execAllowedHosts(flags))
	}
	listener, err := webui.Listen(flags.host, flags.port, !flags.portExplicit)
	if err != nil {
		printErrorf("web server failed: %v", err)
		return 1
	}
	if flags.detached != nil {
		if err := flags.detached.listening(flags, baseDir, listener.Addr(), options.Shutdown); err != nil {
			_ = listener.Close()
			printErrorf("web server failed: %v", err)
			return 1
		}
	}
	if flags.detached == nil || flags.detached.lifeline != nil {
		// A server whose output someone sees: in the foreground, or attached to a client.
		noteDetachedWeb()
	}
	server := &http.Server{Handler: handler}
	if webLog != nil {
		identity, identityErr := currentExecutableIdentity()
		if identityErr != nil {
			printErrorf("WARNING: Web server executable identity not recorded: %v", identityErr)
		}
		host, portText, _ := net.SplitHostPort(listener.Addr().String())
		port, _ := strconv.Atoi(portText)
		auth := "none"
		if flags.authGenerated {
			auth = "generated"
		} else if flags.authToken != "" {
			auth = "flag_or_env"
		}
		webLog.RecordServerStart(weblog.Start{
			Version: versionString(), ExecutableSHA256: identity.SHA256, ExecutableModified: identity.Modified,
			Host: host, Port: port, Auth: auth, AllowExec: flags.allowExec, AllowTerminal: flags.allowTerminal,
			AllowControl: flags.allowControl, Notifications: flags.notifications,
			ArtifactRootCount: len(flags.artifactRoots), RegisteredBaseDirCount: len(options.BaseDirs),
		})
	}
	go func() {
		select {
		case <-interruptSignal():
		case <-shutdownRequested:
		}
		// Stop launches before the listener, so none starts meanwhile.
		if options.Launches != nil {
			_ = options.Launches.Manager.Close()
		}
		// The server does not close the terminal's connection itself; this does.
		if options.Terminal != nil {
			options.Terminal.Close()
		}
		_ = server.Close()
	}()
	fmt.Printf("rotari web listening at http://%s\n", listener.Addr())
	if flags.detached != nil {
		if err := flags.detached.announced(); err != nil {
			printErrorf("web server failed: %v", err)
			options.Shutdown()
		}
	}
	if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		printErrorf("web server failed: %v", err)
		return 1
	}
	return 0
}

// webDisconnectActionDefault is --disconnect-action's default from disconnect-action
// under [web] in a config, then ROTARI_WEB_DISCONNECT_ACTION. A top-level
// disconnect-action, like ROTARI_DISCONNECT_ACTION, is a run's (detach or cancel), so
// it does not apply here.
func webDisconnectActionDefault(fs *flag.FlagSet) string {
	choices := cliCommandFlag("web", "disconnect-action").Values
	action := webDisconnectStop
	apply := func(value configuredValue, text string) {
		chosen, err := value.choice("disconnect-action", text, choices)
		if err != nil {
			addConfiguredProblem(fs, err)
			return
		}
		action = chosen
	}
	section, _ := cliConfig["web"].(map[string]any)
	if raw, ok := section["disconnect-action"]; ok && raw != nil {
		from := "config"
		// The section is looked at first, so this names where [web] set it.
		if setting := configSettingIn(cliConfigLayers, "web", "disconnect-action"); setting.found {
			from = setting.where()
		}
		value := configuredValue{raw: raw, from: from, set: true}
		if text, err := value.scalarText("disconnect-action"); err != nil {
			addConfiguredProblem(fs, err)
		} else {
			apply(value, text)
		}
	}
	if value := configuredFromEnvironment(envWebDisconnectAction); value.set {
		apply(value, value.raw.(string))
	}
	return action
}

// warnUnauthenticatedHost warns when a server without a token listens beyond
// loopback.
func warnUnauthenticatedHost(flags webCommandFlags, token string) {
	if webui.IsLoopbackHost(flags.host) || token != "" {
		return
	}
	controlWarning := "registered basedir paths, job logs, artifact files under job working directories and --artifact-root, and environment variable names"
	if flags.allowControl {
		controlWarning = "registered basedir paths, job logs, artifact files under job working directories and --artifact-root, environment variable names, and job control (cancel/suspend/resume/change/remove/copy) operations"
	}
	printErrorf("WARNING: --host %s exposes %s over unauthenticated HTTP.", flags.host, controlWarning)
}

func webStaticServerOptions(fs *flag.FlagSet) []string {
	return staticServerOptions(fs, "web", append([]string{"allow-control", "allow-exec", "allow-terminal", "allowed-host", "artifact-root", "auth-token", "detach", "disconnect-action", "host", "port", "shutdown", "yes"}, webTerminalOptions...))
}

// staticServerOptions lists the names, options of command that only a live
// server uses, given on the command line, in the environment, or in the
// config. An auth-token from the environment or config is the exception: it
// protects a live server, and a static export has none, so it is ignored
// with a warning rather than refused.
func staticServerOptions(fs *flag.FlagSet, command string, names []string) []string {
	var incompatible []string
	for _, name := range names {
		provided := cliOptionSet(fs, name)
		if name == "auth-token" && !provided {
			warnIgnoredStaticToken(command)
			continue
		}
		spec := cliCommandFlag(command, name)
		if !provided && !spec.CommandLineOnly && command == "web" {
			provided = webOptionConfigured(name)
		} else if !provided && !spec.CommandLineOnly {
			if envName := cliCommandEnvironmentVariable(command, name); envName != "" {
				_, provided = os.LookupEnv(envName)
			}
			if !provided {
				_, provided = configValue(name)
			}
		}
		if provided {
			incompatible = append(incompatible, name)
		}
	}
	return incompatible
}

// warnIgnoredStaticToken says that a static export of command leaves out the
// auth-token the environment or config gives, naming where it is but never
// the token.
func warnIgnoredStaticToken(command string) {
	from := ""
	if environment := configuredFromEnvironment(cliCommandEnvironmentVariable(command, "auth-token")); environment.set {
		from = environment.from
	} else if setting := configuredFromConfig("auth-token"); setting.set {
		from = setting.from
	}
	if from != "" {
		printWarningf("WARNING: ignoring auth-token in %s; a static export has no server to protect", from)
	}
}

func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	return fs
}

func interruptSignal() <-chan os.Signal {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	return signals
}

// webOptions wires the Web UI to this command's state, executors, and CLI
// metadata.
func webOptions(baseDir string, allowControl, notifications bool) webui.Options {
	return webui.Options{
		BaseDir: baseDir, RootBaseDir: baseDir, AllowControl: allowControl, Notifications: notifications,
		Store: jsonStore(), Editor: queueEditor(), Controller: jobController(),
		Executors:              executorRegistry.Names(),
		Environments:           environmentDefinitions(),
		ConfigTemplate:         func() ([]byte, error) { return configTemplate("toml") },
		ConfigTemplateForScope: func(scope string) ([]byte, error) { return configTemplate("toml", scope) },
	}
}

// webAbout says which rotari serves the Web pages, for their Settings menu.
func webAbout() webui.About {
	about := webui.About{Version: versionString()}
	executable, err := os.Executable()
	if err != nil {
		about.ExecutableError = err.Error()
	} else {
		about.Executable = executable
	}
	return about
}
