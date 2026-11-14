package main

import (
	"strings"
	"testing"

	"github.com/rotari-dev/rotari/internal/model"
)

func TestBinaryNote(t *testing.T) {
	current, err := currentBinary()
	if err != nil {
		t.Fatal(err)
	}
	if note := binaryNote("run R is managed by", nil); note != "" {
		t.Errorf("a record without a binary gave %q", note)
	}
	if note := binaryNote("run R is managed by", &current); note != "" {
		t.Errorf("the same binary gave %q", note)
	}
	moved := current
	moved.Executable = "/elsewhere/rotari"
	if note := binaryNote("run R is managed by", &moved); !strings.HasPrefix(note, "Note: run R is managed by a different build of "+current.Version+" (/elsewhere/rotari), not this one (") {
		t.Errorf("another build of this version gave %q", note)
	}
	older := current
	older.Version, older.Size = "rotari v0.0.1", current.Size+1
	if note := binaryNote("run R is managed by", &older); note != "Note: run R is managed by rotari v0.0.1 ("+current.Executable+"), not this "+current.Version+" ("+current.Executable+")." {
		t.Errorf("an older version gave %q", note)
	}
}

func TestRunBinaryDiffers(t *testing.T) {
	base := model.RunBinary{Version: "rotari v1", Executable: "/r", Size: 10, Modified: "2026-11-14T00:00:00Z"}
	for name, change := range map[string]func(*model.RunBinary){
		"path": func(b *model.RunBinary) { b.Executable = "/s" }, "size": func(b *model.RunBinary) { b.Size = 11 },
		"time": func(b *model.RunBinary) { b.Modified = "2026-11-14T00:00:01Z" },
	} {
		other := base
		change(&other)
		if !base.Differs(other) {
			t.Errorf("a different %s did not differ", name)
		}
	}
	version := base
	version.Version = "rotari v2"
	if base.Differs(version) {
		t.Error("the version alone made binaries differ; the file decides")
	}
}
