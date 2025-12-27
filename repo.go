package main

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/go-enry/go-enry/v2"
)

type Repo struct {
	Identifier      string
	Path            string
	VendoredFilters []*regexp.Regexp
	Files           []string
	UniqueFiles     []string
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

func initRepo(repo *Repo) {
	repo.Path = repoPath(repo.Identifier)

	repo.UniqueFiles = []string{}
	repo.FileLangMap = map[string][]string{}
	repo.FileSkipMap = map[string]bool{}
	repo.LogID = -1

	repo.pullOrClone()

	log(Info, repo, fmt.Sprintf("Initialized repository at %s", repo.Path))
}

func (repo *Repo) insertUniqueFile(file string) {
	idx, found := slices.BinarySearch(repo.UniqueFiles, file)

	if !found {
		log(Debug, repo, fmt.Sprintf("Adding unique file %s", file))
		repo.UniqueFiles = slices.Insert(repo.UniqueFiles, idx, file)
	}
}

func (repo *Repo) updateFiles() {
	stdout, _, err := runGitSync(repo.Path, "ls-files")
	check(err)

	repo.Files = strings.Split(stdout, "\n")
}

var vendoredRegexp *regexp.Regexp

func (repo *Repo) vendoredFilters() []*regexp.Regexp {
	if vendoredRegexp == nil {
		vendoredRegexp = regexp.MustCompile(" linguist-vendored")
	}

	gitAttrPath := path.Join(repo.Path, ".gitattributes")

	if !fileExists(gitAttrPath) {
		return []*regexp.Regexp{}
	}

	data, err := os.ReadFile(gitAttrPath)

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
	for i := range lines {
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

func repoPath(repoID string) string {
	splits := strings.Split(repoID, "/")

	if len(splits) != 2 {
		panic(
			fmt.Sprintf(
				"Improper repository provided: %s. Ensure repositories follow the format author/repo",
				repoID,
			),
		)
	}

	return path.Join(config.Location, fmt.Sprintf("%s-%s", splits[0], splits[1]))
}

func (repo *Repo) pullOrClone() {
	var latestBranch string

	if !fileExists(repo.Path) {
		msg := "Cloning repository"
		logProgess(repo, msg, 0)
		log(Info, repo, msg)
		_, _, err := runGitSync("", "clone", "https://github.com/"+repo.Identifier+".git", repo.Path)
		check(err)
	} else {
		msg := fmt.Sprintf("Pulling repository at %s", repo.Path)
		logProgess(repo, msg, 0)
		log(Info, repo, msg)
		_, _, err := runGitSync(repo.Path, "fetch", "origin")

		// TODO: Better handling of empty repositories
		if err != nil && strings.Contains(err.Error(), "no such ref was fetched") {
			logProgess(repo, "Finished (empty repository)", 1)
			return
		} else {
			check(err)
		}

		latestBranch = repo.getCurrentBranch()
		_, _, err = runGitSync(repo.Path, "reset", "--hard", "origin/"+latestBranch)
		check(err)
	}

	repo.VendoredFilters = repo.vendoredFilters()
	repo.updateFiles()

	if len(latestBranch) == 0 {
		latestBranch = repo.getCurrentBranch()
	}

	repo.CurrentBranch = latestBranch
	repo.LatestBranch = latestBranch

	latestCommit := repo.getLatestCommit()
	repo.CurrentCommit = latestCommit
	repo.LatestCommit = latestCommit
}

func (repo *Repo) isPathVendored(path string) bool {
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

func (repo *Repo) shouldSkipFileByName(repoFile string) bool {
	if config.Ignore.Vendor && repo.isPathVendored(repoFile) {
		log(Info, repo, fmt.Sprintf("Skipping vendored file %s", repoFile))
		return true
	}

	if config.Ignore.Dotfiles && enry.IsDotFile(repoFile) {
		log(Info, repo, fmt.Sprintf("Skipping dotfile file %s", repoFile))
		return true
	}

	if config.Ignore.Configuration && enry.IsConfiguration(repoFile) {
		log(Info, repo, fmt.Sprintf("Skipping config file %s", repoFile))
		return true
	}

	if config.Ignore.Image && enry.IsImage(repoFile) {
		log(Info, repo, fmt.Sprintf("Skipping image file %s", repoFile))
		return true
	}

	if config.Ignore.Test && enry.IsTest(repoFile) {
		log(Info, repo, fmt.Sprintf("Skipping test file %s", repoFile))
		return true
	}

	return false
}

func (repo *Repo) skipFileByData(repoFile string, data []byte) bool {
	if config.Ignore.Binary && enry.IsBinary(data) {
		log(Info, repo, fmt.Sprintf("Skipping binary file %s", repoFile))
		return true
	}

	if config.Ignore.Generated && enry.IsGenerated(repoFile, data) {
		log(Info, repo, fmt.Sprintf("Skipping generated file %s", repoFile))
		return true
	}

	return false
}

func (repo *Repo) count() map[string]*IntIntPair {
	ret := map[string]*IntIntPair{}

	flen := float64(len(repo.Files))
	for i, repoFile := range repo.Files {
		if len(repoFile) == 0 {
			continue
		}

		msg := fmt.Sprintf("Counting file %s", repoFile)
		logProgess(repo, msg, float64(i)/flen)
		log(Info, repo, msg)

		fpath := path.Join(repo.Path, repoFile)

		if repo.shouldSkipFileByName(repoFile) {
			continue
		}

		data, err := os.ReadFile(fpath)
		check(err)

		if repo.skipFileByData(repoFile, data) {
			continue
		}

		langs := enry.GetLanguages(repoFile, data)
		if len(langs) > 1 {
			log(Warning, repo, fmt.Sprintf("Potentially multiple languages found for file %s: %s", fpath, langs))
		}

		if len(langs) == 0 {
			langs = append(langs, "Unknown")
		}

		pair := ret[langs[0]]
		if pair == nil {
			pair = &IntIntPair{}
			ret[langs[0]] = pair
		}

		pair.lines += bytes.Count(data, []byte{'\n'})
		pair.bytes += len([]byte(data))
	}

	log(Info, repo, "Finished")
	logProgess(repo, "Finished", 1)

	return ret
}

func (repo *Repo) countByCommit() map[string]*IntIntPair {
	ret := map[string]*IntIntPair{}

	commits := repo.getMatchingCommits()
	clen := float64(len(commits))

	for i, commit := range commits {
		if commit.shouldSkipCommit() {
			log(Info, repo, fmt.Sprintf("Skipping commit %s", commit.Hash))
			continue
		}

		msg := fmt.Sprintf("Checking out commit %s", commit.Hash)
		logProgess(repo, msg, float64(i)/clen)
		log(Info, repo, msg)
		repo.checkoutCommit(commit)

		for _, diff := range commit.getDiffs(repo) {
			if diff.shouldSkip(repo) {
				continue
			}

			langs := diff.getLanguages(repo)
			if len(langs) > 1 {
				log(Warning, repo, fmt.Sprintf("Potentially multiple languages found for file %s: %s", diff.File, langs))
			}

			if len(langs) == 0 {
				langs = append(langs, "Unknown")
			}

			if !shouldSkipLang(langs[0]) {
				repo.insertUniqueFile(diff.File)
			}

			pair := ret[langs[0]]
			if pair == nil {
				pair = &IntIntPair{}
				ret[langs[0]] = pair
			}

			if config.CountTotal {
				pair.lines += diff.Added.lines - diff.Removed.lines
				pair.bytes += diff.Added.bytes - diff.Removed.bytes
			} else {
				pair.lines += diff.Added.lines + diff.Removed.lines
				pair.bytes += diff.Added.bytes + diff.Removed.bytes
			}
		}
	}

	msg := fmt.Sprintf("Checking out branch %s", repo.LatestBranch)
	logProgess(repo, msg, 0.99)
	log(Info, repo, msg)
	repo.checkoutBranch(repo.LatestBranch)

	log(Info, repo, "Finished")
	logProgess(repo, "Finished", 1)

	return ret
}

func (repo *Repo) getMatchingCommits() []Commit {
	ret := []Commit{}

	for _, author := range config.Authors {
		commitsText, _, err := runGitSync(repo.Path, "log", "--author="+author, "--no-merges", "--pretty=format:%h %ct")
		check(err)
		commitsLines := strings.Split(commitsText, "\n")

		for _, line := range commitsLines {
			split := strings.Fields(line)

			if len(split) != 2 {
				continue
			}

			timestamp, err := strconv.ParseUint(split[1], 10, 64)
			check(err)
			ret = commitsInsertSortedUnique(ret, makeCommit(repo, split[0], timestamp))
		}
	}

	return ret
}

func (repo *Repo) getLatestCommit() Commit {
	stdout, _, err := runGitSync(repo.Path, "log", "-n", "1", "--pretty=format:%h %ct")
	if err != nil && strings.Contains(err.Error(), "does not have any commits yet") {
		return Commit{}
	} else {
		check(err)
	}

	commitLine := strings.Trim(stdout, "\t\n\r ")
	split := strings.Fields(commitLine)

	timestamp, err := strconv.ParseUint(split[1], 10, 64)
	check(err)

	return makeCommit(repo, split[0], timestamp)
}

func (repo *Repo) getCurrentBranch() string {
	stdout, _, err := runGitSync(repo.Path, "branch", "--show-current")
	check(err)

	return strings.Trim(stdout, "\n\r\t ")
}

func (repo *Repo) checkoutBranch(branch string) {
	if repo.CurrentBranch == branch {
		return
	}

	_, _, err := runGitSync(repo.Path, "checkout", branch)
	check(err)

	repo.CurrentBranch = branch
	repo.CurrentCommit = repo.getLatestCommit()

	repo.updateFiles()
}

func (repo *Repo) checkoutCommit(commit Commit) {
	if repo.CurrentCommit == commit {
		return
	}

	_, _, err := runGitSync(repo.Path, "checkout", commit.Hash)
	check(err)

	repo.CurrentCommit = commit
	repo.CurrentBranch = ""

	repo.updateFiles()
}
