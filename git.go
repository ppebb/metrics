package main

import (
	"bytes"
	"fmt"
	"os/exec"
)

func run_git_sync(dir string, args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	cmd.Stdout = stdout
	cmd.Stderr = stderr

	check(cmd.Run())

	out := stdout.Bytes()
	err := stderr.Bytes()

	code := cmd.ProcessState.ExitCode()
	if code != 0 {
		panic(fmt.Sprintf("Git errored with code %d: %s", code, string(err)))
	}

	return string(out)
}
