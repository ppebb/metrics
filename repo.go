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
	FileLangMap     map[string][]string
	FileSkipMap     map[string]bool
	CurrentCommit   Commit
	LatestCommit    Commit
	CurrentBranch   string
	LatestBranch    string
	// Bit of a misnomer. This is also used to track the position when
	// outputting to the console. :)
	LogID int
}

func repo_new(repo_id string) Repo {
	ret := Repo{}

	ret.Identifier = repo_id
	ret.Path = repo_path(repo_id)

	ret.FileLangMap = map[string][]string{}
	ret.FileSkipMap = map[string]bool{}
	ret.LogID = -1

	ret.repo_pull_or_clone()

	return ret
}

func repo_files(repo_path string) []string {
	stdout, _, err := run_git_sync(repo_path, "ls-files")
	check(err)

	return strings.Split(stdout, "\n")
}

var vendoredRegexp *regexp.Regexp

func (repo *Repo) vendored_filters() []*regexp.Regexp {
	if vendoredRegexp == nil {
		vendoredRegexp = regexp.MustCompile(" linguist-vendored")
	}

	gitattr_path := path.Join(repo.Path, ".gitattributes")

	if !file_exists(gitattr_path) {
		return []*regexp.Regexp{}
	}

	data, err := os.ReadFile(gitattr_path)

	if err != nil {
		log(Warning, repo, fmt.Sprintf(
			"Unable to read .gitattributes for %s due to error %s, results may be innacurate!",
			repo.Path,
			err.Error(),
		))
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

func (repo *Repo) repo_pull_or_clone() {
	if !file_exists(repo.Path) {
		msg := "Cloning repository"
		log_progress(repo, msg, 0)
		log(Info, repo, msg)
		run_git_sync(config.Location, "clone", "https://github.com/"+repo.Identifier+".git")
	} else {
		msg := fmt.Sprintf("Pulling repository at %s", repo.Path)
		log_progress(repo, msg, 0)
		log(Info, repo, msg)
		run_git_sync(repo.Path, "pull")
	}

	repo.VendoredFilters = repo.vendored_filters()
	repo.Files = repo_files(repo.Path)

	latestBranch := repo.get_current_branch()
	repo.CurrentBranch = latestBranch
	repo.LatestBranch = latestBranch

	latestCommit := repo.get_latest_commit()
	repo.CurrentCommit = latestCommit
	repo.LatestCommit = latestCommit
}

func (repo *Repo) refresh() {
	// repo.VendoredFilters = repo.vendored_filters()
	repo.Files = repo_files(repo.Path)
}

func (repo *Repo) check_path_vendored(path string) bool {
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

func (repo *Repo) skip_file_name(repo_file string, fpath string) bool {
	if !file_exists(fpath) || is_symlink(fpath) || is_directory(fpath) {
		return true
	}

	if config.Ignore.Vendor && repo.check_path_vendored(repo_file) {
		log(Info, repo, fmt.Sprintf("Skipping vendored file %s", repo_file))
		return true
	}

	if config.Ignore.Dotfiles && enry.IsDotFile(repo_file) {
		log(Info, repo, fmt.Sprintf("Skipping dotfile file %s", repo_file))
		return true
	}

	if config.Ignore.Configuration && enry.IsConfiguration(repo_file) {
		log(Info, repo, fmt.Sprintf("Skipping config file %s", repo_file))
		return true
	}

	if config.Ignore.Image && enry.IsImage(repo_file) {
		log(Info, repo, fmt.Sprintf("Skipping image file %s", repo_file))
		return true
	}

	if config.Ignore.Test && enry.IsTest(repo_file) {
		log(Info, repo, fmt.Sprintf("Skipping test file %s", repo_file))
		return true
	}

	return false
}

func (repo *Repo) skip_file_data(repo_file string, data []byte) bool {
	if config.Ignore.Binary && enry.IsBinary(data) {
		log(Info, repo, fmt.Sprintf("Skipping binary file %s", repo_file))
		return true
	}

	if config.Ignore.Generated && enry.IsGenerated(repo_file, data) {
		log(Info, repo, fmt.Sprintf("Skipping generated file %s", repo_file))
		return true
	}

	return false
}

func (repo *Repo) repo_count(lines bool) map[string]int {
	ret := map[string]int{}

	flen := float64(len(repo.Files))
	for i, repo_file := range repo.Files {
		msg := fmt.Sprintf("Counting file %s", repo_file)
		log_progress(repo, msg, float64(i)/flen)
		log(Info, repo, msg)

		fpath := path.Join(repo.Path, repo_file)

		if repo.skip_file_name(repo_file, fpath) {
			continue
		}

		data, err := os.ReadFile(fpath)
		check(err)

		if repo.skip_file_data(repo_file, data) {
			continue
		}

		langs := enry.GetLanguages(repo_file, data)
		if len(langs) > 1 {
			log(Warning, repo, fmt.Sprintf("Potentially multiple languages found for file %s: %s", fpath, langs))
		}

		if len(langs) == 0 {
			langs = append(langs, "Unknown")
		}

		if lines {
			ret[langs[0]] += bytes.Count(data, []byte{'\n'})
		} else {
			ret[langs[0]] += len(data)
		}
	}

	log(Info, repo, "Finished")
	log_progress(repo, "Finished", 1)

	return ret
}

func (repo *Repo) repo_count_by_commit(lines bool) map[string]int {
	ret := map[string]int{}

	commits := repo.get_matching_commits()
	clen := float64(len(commits))
	for i, commit := range commits {
		msg := fmt.Sprintf("Checking out commit %s", commit.Hash)
		log_progress(repo, msg, float64(i)/clen)
		log(Info, repo, msg)
		repo.checkout_commit(commit)

		var diffs []Diff
		if lines {
			diffs = commit.get_diffs_lines(repo)
		} else {
			diffs = commit.get_diffs_bytes(repo)
		}

		for _, diff := range diffs {
			if diff.should_skip(repo) {
				continue
			}

			langs := diff.get_languages(repo)
			if len(langs) > 1 {
				log(Warning, repo, fmt.Sprintf("Potentially multiple languages found for file %s: %s", diff.File, langs))
			}

			if len(langs) == 0 {
				langs = append(langs, "Unknown")
			}

			if config.CountTotal {
				ret[langs[0]] += int(diff.Added - diff.Removed)
			} else {
				ret[langs[0]] += int(diff.Added + diff.Removed)
			}
		}
	}

	msg := fmt.Sprintf("Checking out branch %s", repo.LatestBranch)
	log_progress(repo, msg, 0.99)
	log(Info, repo, msg)
	repo.checkout_branch(repo.LatestBranch)

	log(Info, repo, "Finished")
	log_progress(repo, "Finished", 1)

	return ret
}

func (repo *Repo) get_matching_commits() []Commit {
	ret := []Commit{}

	for _, author := range config.Authors {
		commits_text, _, err := run_git_sync(repo.Path, "log", "--author="+author, "--no-merges", "--pretty=format:%h %ct")
		check(err)
		commits_lines := strings.Split(commits_text, "\n")

		for _, line := range commits_lines {
			split := strings.Fields(line)

			if len(split) != 2 {
				continue
			}

			timestamp, err := strconv.ParseUint(split[1], 10, 64)
			check(err)
			ret = commits_insert_sorted_unique(ret, commit_new(repo, split[0], timestamp))
		}
	}

	return ret
}

func (repo *Repo) get_latest_commit() Commit {
	stdout, _, err := run_git_sync(repo.Path, "log", "-n", "1", "--pretty=format:%h %ct")
	check(err)

	commit_line := strings.Trim(stdout, "\t\n\r ")
	split := strings.Fields(commit_line)

	timestamp, err := strconv.ParseUint(split[1], 10, 64)
	check(err)

	return commit_new(repo, split[0], timestamp)
}

func (repo *Repo) get_current_branch() string {
	stdout, _, err := run_git_sync(repo.Path, "branch", "--show-current")
	check(err)

	return strings.Trim(stdout, "\n\r\t ")
}

func (repo *Repo) checkout_branch(branch string) {
	if repo.CurrentBranch == branch {
		return
	}

	_, _, err := run_git_sync(repo.Path, "checkout", branch)
	check(err)

	repo.CurrentBranch = branch
	repo.CurrentCommit = repo.get_latest_commit()

	repo.refresh()
}

func (repo *Repo) checkout_commit(commit Commit) {
	if repo.CurrentCommit == commit {
		return
	}

	_, _, err := run_git_sync(repo.Path, "checkout", commit.Hash)
	check(err)

	repo.CurrentCommit = commit
	repo.CurrentBranch = ""

	repo.refresh()
}
