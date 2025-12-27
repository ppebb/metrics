package main

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

type Commit struct {
	Hash      string
	Timestamp uint64
	Root      bool
}

func makeCommit(repo *Repo, hash string, timestamp uint64) Commit {
	return Commit{
		Hash:      hash,
		Timestamp: timestamp,
		Root:      commitIsRoot(repo, hash),
	}
}

func commitIsRoot(repo *Repo, hash string) bool {
	_, stderr, err := runGitSync(repo.Path, "rev-parse", hash+"^")

	if strings.Contains(stderr, "unknown revision or path not in the working tree") {
		return true
	} else if err == nil {
		return false
	}

	panic(err.Error())
}

func compareCommit(c1 Commit, c2 Commit) int {
	return cmp.Compare(c1.Timestamp, c2.Timestamp)
}

func commitsInsertSortedUnique(commits []Commit, commit Commit) []Commit {
	idx, found := slices.BinarySearchFunc(commits, commit, compareCommit)

	if !found {
		return slices.Insert(commits, idx, commit)
	}

	return commits
}

func getBytesForFileHash(hash string, repo *Repo) (int, error) {
	stdout, _, err := runGitSync(repo.Path, "cat-file", "-s", hash)
	if err != nil && strings.Contains(err.Error(), "could not get object info") {
		return 0, nil
	}
	check(err)

	stdout = strings.ReplaceAll(stdout, "\n", "")

	return strconv.Atoi(stdout)
}

func iabs(n int) int {
	if n < 0 {
		return -n
	}

	return n
}

func (commit Commit) getDiffs(repo *Repo) []Diff {
	ret := []Diff{}

	var prevCommit string
	if commit.Root {
		prevCommit = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
	} else {
		prevCommit = "HEAD^"
	}

	stdout, _, err := runGitSync(repo.Path, "diff", "--patch", prevCommit, commit.Hash)
	check(err)

	diffLines := strings.Split(stdout, "\n")

	var currentDiff Diff

	minLen := 2

	if config.CountSpaces {
		minLen = 1
	}

	for _, line := range diffLines {
		// Minimum viable line is a +/- followed by any other character
		if len(line) < minLen {
			continue
		}

		if stringBeginsWith(line, "diff") {
			start := strings.Index(line, "a/")
			end := strings.Index(line, "b/")

			if start == -1 || end == -1 {
				log(Warning, repo, fmt.Sprintf("Patch line contained 'diff', but a/ was at %d and b/ was at %d", start, end))
				continue
			}

			if len(currentDiff.File) != 0 {
				ret = append(ret, currentDiff)
			}

			currentDiff = Diff{
				File:    line[start+2 : end-1],
				Added:   IntIntPair{},
				Removed: IntIntPair{},
			}

			continue
		}

		if stringBeginsWith(line, "rename to") {
			start := len("rename to ")
			end := len(line)

			if start == -1 || end == -1 {
				log(Warning, repo, fmt.Sprintf("Patch line contained 'diff', but a/ was at %d and b/ was at %d", start, end))
				continue
			}

			currentDiff.File = line[start:end]
		}

		c0 := line[0]
		c1 := line[1:]

		switch c0 {
		case '+':
			currentDiff.Added.lines++
			currentDiff.Added.bytes += len([]byte(c1))
		case '-':
			currentDiff.Removed.lines++
			currentDiff.Removed.bytes += len([]byte(c1))
		default:
			continue
		}
	}

	return append(ret, currentDiff)
}

func (commit Commit) shouldSkipCommit() bool {
	for _, filteredHash := range config.Commits {
		if strings.HasPrefix(commit.Hash, filteredHash) {
			return true
		}
	}

	return false
}
