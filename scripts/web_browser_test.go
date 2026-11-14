package scripts_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

// fakeChrome creates DevToolsActivePort empty and fills it later, so a reader
// that goes by the file existing sees it with nothing in it.
const fakeChrome = `#!/bin/sh
for argument in "$@"; do
  case "$argument" in
    --user-data-dir=*) profile="${argument#*=}" ;;
  esac
done
: > "$profile/DevToolsActivePort"
sleep 0.5
printf '%s\n/devtools/browser/fake' "$FAKE_DEVTOOLS_PORT" > "$profile/DevToolsActivePort"
exec sleep 60
`

// A protocol endpoint that only accepts the connection, which is all launching needs.
const launchCheck = `
import { createHash } from "node:crypto";
import { createServer } from "node:http";
import { pathToFileURL } from "node:url";

const [helper, chrome] = process.argv.slice(1);
const { launchChrome } = await import(pathToFileURL(helper));
const server = createServer();
server.on("upgrade", (request, socket) => {
  const accept = createHash("sha1")
    .update(request.headers["sec-websocket-key"] + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11")
    .digest("base64");
  socket.write("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: " + accept + "\r\n\r\n");
});
await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
process.env.FAKE_DEVTOOLS_PORT = String(server.address().port);
const { close } = await launchChrome(chrome);
await close();
process.exit(0);
`

// TestLaunchChromeWaitsForDevToolsPortContents covers a browser whose port file
// exists before it has anything in it, which failed browser checks with
// "Invalid URL" on a loaded host.
func TestLaunchChromeWaitsForDevToolsPortContents(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil || exec.Command(node, "-e", `if (typeof WebSocket !== "function") process.exit(1)`).Run() != nil {
		if os.Getenv("ROTARI_REQUIRE_BROWSER_TESTS") != "" {
			t.Fatal("launching the browser needs Node 22 or later")
		}
		t.Skip("launching the browser needs Node 22 or later")
	}
	helper, err := filepath.Abs("web-browser.mjs")
	if err != nil {
		t.Fatal(err)
	}
	chrome := filepath.Join(t.TempDir(), "chrome")
	if err := os.WriteFile(chrome, []byte(fakeChrome), 0o700); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(node, "--input-type=module", "-e", launchCheck, helper, chrome).CombinedOutput(); err != nil {
		t.Fatalf("launching: %v\n%s", err, output)
	}
}

// slowExitChrome is ready at once and, on SIGTERM, keeps writing into its
// profile for a second before it exits, as Chrome does while it shuts down.
// It records its profile and PID in $FAKE_RECORD.
const slowExitChrome = `#!/bin/sh
for argument in "$@"; do
  case "$argument" in
    --user-data-dir=*) profile="${argument#*=}" ;;
  esac
done
printf '%s\n%s\n' "$profile" "$$" > "$FAKE_RECORD"
printf '%s\n/devtools/browser/fake' "$FAKE_DEVTOOLS_PORT" > "$profile/DevToolsActivePort"
stopping=
trap 'stopping=1' TERM
while [ -z "$stopping" ]; do sleep 0.05; done
i=0
while [ "$i" -lt 20 ]; do
  mkdir -p "$profile/Default" && : > "$profile/Default/late-$i"
  i=$((i + 1))
  sleep 0.05
done
`

// TestCloseWaitsForTheBrowserToExit covers a browser still writing its
// profile after SIGTERM: close must return only once the browser has exited
// and its profile is removed. A fixed 300 ms wait failed browser checks in
// CI with ENOTEMPTY while removing the profile.
func TestCloseWaitsForTheBrowserToExit(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil || exec.Command(node, "-e", `if (typeof WebSocket !== "function") process.exit(1)`).Run() != nil {
		if os.Getenv("ROTARI_REQUIRE_BROWSER_TESTS") != "" {
			t.Fatal("launching the browser needs Node 22 or later")
		}
		t.Skip("launching the browser needs Node 22 or later")
	}
	helper, err := filepath.Abs("web-browser.mjs")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	chrome, record := filepath.Join(dir, "chrome"), filepath.Join(dir, "record")
	if err := os.WriteFile(chrome, []byte(slowExitChrome), 0o700); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(node, "--input-type=module", "-e", launchCheck, helper, chrome)
	command.Env = append(os.Environ(), "FAKE_RECORD="+record)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("launching and closing: %v\n%s", err, output)
	}
	data, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	fields := strings.Fields(string(data))
	if len(fields) != 2 {
		t.Fatalf("record = %q", data)
	}
	profile, pid := fields[0], fields[1]
	if number, err := strconv.Atoi(pid); err != nil {
		t.Fatalf("record PID %q: %v", pid, err)
	} else if err := syscall.Kill(number, 0); err == nil || errors.Is(err, syscall.EPERM) {
		t.Errorf("close returned while the browser (PID %s) still ran", pid)
	}
	if _, err := os.Stat(profile); !os.IsNotExist(err) {
		t.Errorf("the profile %s remains after close: %v", profile, err)
	}
}
