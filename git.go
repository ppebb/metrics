package main

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
)

func run_git_sync(dir string, args ...string) (string, string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	cmd.Stdout = stdout
	cmd.Stderr = stderr

	cmd.Run()

	out := string(stdout.Bytes())
	err := string(stderr.Bytes())

	code := cmd.ProcessState.ExitCode()
	if code != 0 {
		return out, err, errors.New(fmt.Sprintf("Git errored with code %d: %s", code, string(err)))
	}

	return out, err, nil
}
