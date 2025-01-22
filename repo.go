package main

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/go-enry/go-enry/v2"
)

type Repo struct {
	Identifier      string
	Path            string
	VendoredFilters []*regexp.Regexp
	Files           []string
}

func repo_new(repo_id string) Repo {
	ret := Repo{}

	ret.Identifier = repo_id
	ret.Path = repo_path(repo_id)
	ret.VendoredFilters = repo_vendored_filters(ret.Path)
	ret.Files = repo_files(ret.Path)

	return ret
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

func repo_path(repo_id string) string {
	splits := strings.Split(repo_id, "/")

	if len(splits) != 2 {
		panic(
			fmt.Sprintf(
				"Improper repository provided: %s. Ensure repositories follow the format author/repo",
				repo_id,
			),
		)
	}

	return path.Join(config.Location, splits[1])
}

func repo_refresh(repo Repo) {
	if !file_exists(repo.Path) {
		fmt.Printf("Cloning repository %s\n", repo.Identifier)
		run_git_sync(config.Location, "clone", "https://github.com/"+repo.Identifier)
	} else {
		fmt.Printf("Pulling repository %s at %s\n", repo.Identifier, repo.Path)
		run_git_sync(repo.Path, "pull")
	}
}

func repo_check_path_vendored(repo Repo, path string) bool {
	if enry.IsVendor(path) {
		return true
	}

	for _, filter := range repo.VendoredFilters {
		if filter.MatchString(path) {
			return true
		}
	}

	return false
}

func repo_skip_file_name(repo Repo, repo_file string, fpath string) bool {
	if !file_exists(fpath) || is_symlink(fpath) || is_directory(fpath) {
		return true
	}

	if config.Ignore.Vendor && repo_check_path_vendored(repo, repo_file) {
		fmt.Printf("Skipping vendored file %s\n", repo_file)
		return true
	}

	if config.Ignore.Dotfiles && enry.IsDotFile(repo_file) {
		fmt.Printf("Skipping dotfile file %s\n", repo_file)
		return true
	}

	if config.Ignore.Configuration && enry.IsConfiguration(repo_file) {
		fmt.Printf("Skipping config file %s\n", repo_file)
		return true
	}

	if config.Ignore.Image && enry.IsImage(repo_file) {
		fmt.Printf("Skipping image file %s\n", repo_file)
		return true
	}

	if config.Ignore.Test && enry.IsTest(repo_file) {
		fmt.Printf("Skipping test file %s\n", repo_file)
		return true
	}

	return false
}

func repo_skip_file_data(repo_file string, data []byte) bool {
	if config.Ignore.Binary && enry.IsBinary(data) {
		fmt.Printf("Skipping binary file %s\n", repo_file)
		return true
	}

	if config.Ignore.Generated && enry.IsGenerated(repo_file, data) {
		fmt.Printf("Skipping generated file %s\n", repo_file)
		return true
	}

	return false
}

func repo_check(repo Repo) map[string]int {
	ret := map[string]int{}
	for _, repo_file := range repo.Files {
		fpath := path.Join(repo.Path, repo_file)

		if repo_skip_file_name(repo, repo_file, fpath) {
			continue
		}

		data, err := os.ReadFile(fpath)
		check(err)

		if repo_skip_file_data(repo_file, data) {
			continue
		}

		langs := enry.GetLanguages(repo_file, data)
		if len(langs) > 1 {
			fmt.Printf("Potentially multiple languages found for file %s: %s\n", fpath, langs)
		}

		ret[langs[0]] += bytes.Count(data, []byte{'\n'})
	}

	return ret
}
