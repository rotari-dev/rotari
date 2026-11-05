package scripts_test

import (
	"os"
	"os/exec"
	"path/filepath"
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
