package main

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestMain lets the tests run this binary as the csw command.
func TestMain(m *testing.M) {
	if os.Getenv("CSW_TEST_RUN_MAIN") == "1" {
		main()
		return
	}
	os.Exit(m.Run())
}

func TestExitCodes(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		args []string
		code int
		out  string
	}{
		{[]string{"help"}, 0, "usage: csw"},
		{[]string{"frobnicate"}, 2, ""},
		{[]string{"check", t.TempDir()}, 1, ""},
	} {
		cmd := exec.Command(os.Args[0], tt.args...) //nolint:gosec // re-runs this test binary
		cmd.Env = append(os.Environ(), "CSW_TEST_RUN_MAIN=1")
		out, err := cmd.CombinedOutput()
		code := 0
		if ee := new(exec.ExitError); errors.As(err, &ee) {
			code = ee.ExitCode()
		} else if err != nil {
			t.Fatal(err)
		}
		if code != tt.code || !strings.Contains(string(out), tt.out) {
			t.Errorf("%v: exit %d, output %q; want exit %d containing %q", tt.args, code, out, tt.code, tt.out)
		}
	}
}
