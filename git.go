package main

import (
	"bytes"
	"fmt"
	"os/exec"
)

func runGitSync(dir string, args ...string) (string, string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	cmd.Stdout = stdout
	cmd.Stderr = stderr

	cmd.Run()

	out := stdout.String()
	err := stderr.String()

	code := cmd.ProcessState.ExitCode()
	if code != 0 {
		return out, err, fmt.Errorf("git errored with code %d: %s", code, string(err))
	}

	return out, err, nil
}
