package terminal

import (
	"strings"
	"testing"
)

func TestProgramName(t *testing.T) {
	for _, test := range []struct {
		args []string
		want string
	}{
		{nil, ""},
		{[]string{""}, ""},
		{[]string{"-"}, ""},
		{[]string{"sleep", "30"}, "sleep"},
		{[]string{"/usr/bin/python3", "./train.py", "--token", "secret"}, "python3"},
		{[]string{"/opt/my tool/run it"}, "run it"},
		{[]string{"-zsh"}, "zsh"},
		{[]string{"a\x1b[31m\nb"}, "a\ufffd[31m\ufffdb"},
		{[]string{"a\u202eb\u2066c"}, "a\ufffdb\ufffdc"},
		{[]string{"a\xff\xfeb"}, "a\ufffdb"},
		{[]string{"日本"}, "日本"},
		{[]string{strings.Repeat("a", 64)}, strings.Repeat("a", 64)},
		{[]string{strings.Repeat("a", 65)}, strings.Repeat("a", 64)},
		{[]string{strings.Repeat("a", 63) + "日"}, strings.Repeat("a", 63)},
	} {
		if got := programName(test.args); got != test.want {
			t.Errorf("programName(%q) = %q, want %q", test.args, got, test.want)
		}
	}
}
