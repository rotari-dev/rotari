package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/rotari-dev/rotari/internal/state"
	"github.com/rotari-dev/rotari/internal/weblog"
	"github.com/rotari-dev/rotari/internal/webui"
)

// cmdTableView serves the Web UI's Table viewer for one delimited file or
// directory, outside any project, or writes a static snapshot of it for a
// file.
func cmdTableView(args []string) int {
	fs := newFlagSet("tableview")
	host := cliString(fs, "host", "127.0.0.1")
	port := cliInt(fs, "port", webui.TableViewPort)
	authToken := cliString(fs, "auth-token", "")
	staticDir := cliString(fs, "static-dir", "")
	if err := cliParse(fs, args); err != nil {
		return 1
	}
	if len(fs.Args()) != 1 || *port < 0 || *port > 65535 {
		printError("usage: " + cliUsage("tableview"))
		return 1
	}
	if *staticDir != "" {
		if serverOptions := staticServerOptions(fs, "tableview", []string{"auth-token", "host", "port"}); len(serverOptions) > 0 {
			printErrorf("--%s cannot be combined with --static-dir; these options only apply to the live server", strings.Join(serverOptions, ", --"))
			return 1
		}
		copied, err := webui.GenerateStaticTableView(fs.Args()[0], *staticDir)
		if err != nil {
			printErrorf("cannot write a static snapshot of %s: %v", fs.Args()[0], err)
			return 1
		}
		fmt.Printf("Wrote a static snapshot of %s to %s: %d rows and %d media files (%d bytes); anyone who can read it can read them.\n",
			fs.Args()[0], *staticDir, copied.Rows, copied.MediaFiles, copied.MediaBytes)
		return 0
	}
	portExplicit := false
	fs.Visit(func(flag *flag.Flag) { portExplicit = portExplicit || flag.Name == "port" })
	path := fs.Args()[0]
	handler, err := webui.TableViewHandler(path)
	if err != nil {
		printErrorf("cannot view %s: %v", path, err)
		return 1
	}
	masterDir, masterErr := state.ResolveMasterDir("")
	var webLog *weblog.Recorder
	if masterErr != nil {
		printErrorf("WARNING: cannot record Web events: %v", masterErr)
	} else {
		webLog = weblog.New(masterDir, "tableview", os.Stderr)
		defer webLog.Close()
	}
	if *authToken != "" {
		handler = webui.WithAuthToken(handler, *authToken, webui.AuthOptions{Recorder: webLog})
	} else if !webui.IsLoopbackHost(*host) {
		printErrorf("WARNING: --host %s exposes the files under %s over unauthenticated HTTP.", *host, path)
	}
	listener, err := webui.Listen(*host, *port, !portExplicit)
	if err != nil {
		printErrorf("tableview server failed: %v", err)
		return 1
	}
	server := &http.Server{Handler: handler}
	if webLog != nil {
		identity, identityErr := currentExecutableIdentity()
		if identityErr != nil {
			printErrorf("WARNING: Table viewer executable identity not recorded: %v", identityErr)
		}
		host, portText, _ := net.SplitHostPort(listener.Addr().String())
		port, _ := strconv.Atoi(portText)
		auth := "none"
		if *authToken != "" {
			auth = "flag_or_env"
		}
		webLog.RecordServerStart(weblog.Start{Version: versionString(), ExecutableSHA256: identity.SHA256, ExecutableModified: identity.Modified, Host: host, Port: port, Auth: auth})
	}
	go func() {
		<-interruptSignal()
		_ = server.Close()
	}()
	fmt.Printf("rotari tableview listening at http://%s for %s\n", listener.Addr(), path)
	if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		printErrorf("tableview server failed: %v", err)
		return 1
	}
	return 0
}
