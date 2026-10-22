package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestCommandRouting(t *testing.T) {
	for _, command := range []string{"build", "publish", "watch", "dev", "stats", "report"} {
		t.Run(command, func(t *testing.T) {
			var out, errOut bytes.Buffer
			if code := run([]string{command, "--help"}, &out, &errOut); code != 0 || !strings.Contains(out.String(), command) {
				t.Fatalf("help: code=%d out=%q err=%q", code, &out, &errOut)
			}
			out.Reset()
			if code := run([]string{command}, &out, &errOut); code == 0 || errOut.Len() == 0 {
				t.Fatalf("unfinished command: code=%d err=%q", code, &errOut)
			}
		})
	}
	for _, args := range [][]string{nil, {"--help"}, {"unknown"}, {"build", "--invalid"}} {
		var out, errOut bytes.Buffer
		code := run(args, &out, &errOut)
		wantOK := len(args) == 1 && args[0] == "--help"
		if (code == 0) != wantOK {
			t.Fatalf("args %v: exit %d", args, code)
		}
	}
}
