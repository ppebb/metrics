package main

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-enry/go-enry/v2"
)

type Repo struct {
	Identifier      string
	Path            string
	VendoredFilters []*regexp.Regexp
	Files           []string
	FileLangMap     map[string]string
	FileSkipMap     map[string]bool
	CurrentCommit   Commit
	LatestCommit    Commit
	CurrentBranch   string
	LatestBranch    string
}

func repo_new(repo_id string) Repo {
	ret := Repo{}

	ret.Identifier = repo_id
	ret.Path = repo_path(repo_id)

	ret.FileLangMap = map[string]string{}
	ret.FileSkipMap = map[string]bool{}

	repo_pull_or_clone(&ret)

	return ret
}

func repo_files(repo_path string) []string {
	stdout, _, err := run_git_sync(repo_path, "ls-files")
	check(err)

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

func repo_pull_or_clone(repo *Repo) {
	if !file_exists(repo.Path) {
		fmt.Printf("Cloning repository %s\n", repo.Identifier)
		run_git_sync(config.Location, "clone", "https://github.com/"+repo.Identifier+".git")
	} else {
		fmt.Printf("Pulling repository %s at %s\n", repo.Identifier, repo.Path)
		run_git_sync(repo.Path, "pull")
	}

	repo.VendoredFilters = repo_vendored_filters(repo.Path)
	repo.Files = repo_files(repo.Path)

	latestBranch := repo_get_current_branch(*repo)
	repo.CurrentBranch = latestBranch
	repo.LatestBranch = latestBranch

	latestCommit := repo_get_latest_commit(*repo)
	repo.CurrentCommit = latestCommit
	repo.LatestCommit = latestCommit
}

func repo_refresh(repo *Repo) {
	repo.VendoredFilters = repo_vendored_filters(repo.Path)
	repo.Files = repo_files(repo.Path)
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

func repo_count(repo *Repo) map[string]int {
	ret := map[string]int{}

	for _, repo_file := range repo.Files {
		fpath := path.Join(repo.Path, repo_file)

		if repo_skip_file_name(*repo, repo_file, fpath) {
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

func repo_count_by_commit(repo *Repo) map[string]int {
	ret := map[string]int{}

	for _, commit := range repo_get_matching_commits(*repo) {
		fmt.Printf("Checking out commit %s\n", commit.Hash)
		repo_checkout_commit(repo, commit)

		for _, diff := range commit_diffs(commit, *repo) {
			if diff_should_skip(*repo, diff) {
				continue
			}

			lang := diff_get_language(*repo, diff)

			if config.Countloc {
				ret[lang] += int(diff.Added - diff.Removed)
			} else {
				ret[lang] += int(diff.Added + diff.Removed)
			}
		}
	}

	fmt.Printf("Checking out branch %s\n", repo.LatestBranch)
	repo_checkout_branch(repo, repo.LatestBranch)

	return ret
}

func repo_get_matching_commits(repo Repo) []Commit {
	ret := []Commit{}

	for _, author := range config.Authors {
		commits_text, _, err := run_git_sync(repo.Path, "log", "--author="+author, "--no-merges", "--pretty=format:%h %ct")
		check(err)
		commits_lines := strings.Split(commits_text, "\n")

		for _, line := range commits_lines {
			split := strings.Fields(line)

			if len(split) != 2 {
				continue
				// panic(fmt.Sprintf("Commit line %s did not split into 2 strings!", line))
			}

			timestamp, err := strconv.ParseUint(split[1], 10, 64)
			check(err)
			ret = commits_insert_sorted_unique(ret, commit_new(repo, split[0], timestamp))
		}
	}

	return ret
}

func repo_get_latest_commit(repo Repo) Commit {
	stdout, _, err := run_git_sync(repo.Path, "log", "-n", "1", "--pretty=format:%h %ct")
	check(err)

	commit_line := strings.Trim(stdout, "\t\n\r ")
	split := strings.Fields(commit_line)

	timestamp, err := strconv.ParseUint(split[1], 10, 64)
	check(err)

	return commit_new(repo, split[0], timestamp)
}

func repo_get_current_branch(repo Repo) string {
	stdout, _, err := run_git_sync(repo.Path, "branch", "--show-current")
	check(err)

	return strings.Trim(stdout, "\n\r\t ")
}

func repo_checkout_branch(repo *Repo, branch string) {
	if repo.CurrentBranch == branch {
		return
	}

	_, _, err := run_git_sync(repo.Path, "checkout", branch)
	check(err)

	repo.CurrentBranch = branch
	repo.CurrentCommit = repo_get_latest_commit(*repo)

	repo_refresh(repo)
}

func repo_checkout_commit(repo *Repo, commit Commit) {
	if repo.CurrentCommit == commit {
		return
	}

	_, _, err := run_git_sync(repo.Path, "checkout", commit.Hash)
	check(err)

	repo.CurrentCommit = commit
	repo.CurrentBranch = ""

	repo_refresh(repo)
}
