package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"

	"github.com/go-enry/go-enry/v2"
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

func repo_files(repo_path string) []string {
	stdout := run_git_sync(repo_path, "ls-files")

	return strings.Split(stdout, "\n")
}

var vendoredRegexp *regexp.Regexp

func repo_vendored_filters(repo_path string) []*regexp.Regexp {
	if vendoredRegexp == nil {
		vendoredRegexp = regexp.MustCompile(" linguist-vendored")
	}

	gitattr_path := path.Join(repo_path, ".gitattributes")

	if !file_exists(gitattr_path) {
		return []*regexp.Regexp{}
	}

	data, err := os.ReadFile(gitattr_path)

	if err != nil {
		fmt.Fprintf(
			os.Stderr,
			"Unable to read .gitattributes for %s due to error %s, results may be innacurate!\n",
			repo_path,
			err.Error(),
		)
		return []*regexp.Regexp{}
	}

	filters := []*regexp.Regexp{}

	lines := strings.Split(string(data), "\n")
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if vendoredRegexp.MatchString(line) {
			split := strings.Fields(line)[0]
			split = strings.ReplaceAll(split, ".", "\\.")
			split = strings.ReplaceAll(split, "*", ".*")
			filters = append(filters, regexp.MustCompile(split))
		}
	}

	return filters
}

func repo_check_path_vendored(path string, filters []*regexp.Regexp) bool {
	if enry.IsVendor(path) {
		return true
	}

	for _, filter := range filters {
		if filter.MatchString(path) {
			return true
		}
	}

	return false
}
