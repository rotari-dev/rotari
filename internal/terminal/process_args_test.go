package terminal

import (
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func TestParseDarwinProcessArgs(t *testing.T) {
	record := func(count uint32, executable string, arguments, environment []string) []byte {
		data := make([]byte, 4)
		binary.LittleEndian.PutUint32(data, count)
		data = append(data, executable...)
		data = append(data, 0)
		for (len(data)-4)%8 != 0 {
			data = append(data, 0)
		}
		for _, part := range append(arguments, environment...) {
			data = append(data, part...)
			data = append(data, 0)
		}
		return data
	}
	badPadding := record(1, "/bin/bash", []string{"bash"}, nil)
	badPadding[4+len("/bin/bash")+1] = 'x'
	for _, test := range []struct {
		name string
		data []byte
		want []string
	}{
		{"arguments and environment", record(3, "/bin/bash", []string{"-bash", "--init-file", "/tmp/file with spaces"}, []string{"SECRET=not-an-argument"}), []string{"-bash", "--init-file", "/tmp/file with spaces"}},
		{"empty argument", record(3, "/bin/sh", []string{"sh", "", "-i"}, []string{"SECRET=not-an-argument"}), []string{"sh", "", "-i"}},
		{"empty argv0", record(2, "/bin/bash", []string{"", "bash"}, []string{"SECRET=not-an-argument"}), []string{"", "bash"}},
		{"short header", []byte{1, 0, 0}, nil},
		{"zero arguments", record(0, "/bin/sh", []string{"sh"}, nil), nil},
		{"missing executable", append([]byte{1, 0, 0, 0}, 0), nil},
		{"bad padding", badPadding, nil},
		{"missing arguments", record(1, "/bin/sh", nil, nil), nil},
		{"truncated argument", append(record(1, "/bin/sh", nil, nil), 's', 'h'), nil},
		{"excessive count", record(99, "/bin/sh", []string{"sh"}, nil), nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseDarwinProcessArgs(test.data)
			if test.want == nil {
				if err == nil {
					t.Fatalf("parse returned %q without an error", got)
				}
				return
			}
			if err != nil || !slices.Equal(got, test.want) {
				t.Fatalf("parse = %q, %v; want %q", got, err, test.want)
			}
		})
	}
}

func TestProcessArgsPreserveBoundaries(t *testing.T) {
	ready := filepath.Join(t.TempDir(), "ready")
	script := `: > "$1"; while :; do :; done`
	command := exec.Command("/bin/sh", "-c", script, "probe", ready, "argument with spaces", "", "last")
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = command.Process.Kill()
		_ = command.Wait()
	})
	for deadline := time.Now().Add(testWait); ; time.Sleep(10 * time.Millisecond) {
		if _, err := os.Stat(ready); err == nil {
			break
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
		if time.Now().After(deadline) {
			t.Fatal("the test process did not start")
		}
	}
	args, err := processArgs(command.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(args, command.Args) {
		t.Errorf("process args = %q, want %q", args, command.Args)
	}
}
