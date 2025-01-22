package main

import (
	"slices"
	"strconv"
	"strings"
)

type Commit struct {
	Hash      string
	Timestamp uint64
	Root      bool
}

func commit_new(repo Repo, hash string, timestamp uint64) Commit {
	return Commit{
		Hash:      hash,
		Timestamp: timestamp,
		Root:      commit_is_root(repo, hash),
	}
}

func commit_is_root(repo Repo, hash string) bool {
	_, stderr, err := run_git_sync(repo.Path, "rev-parse", hash+"^")

	if strings.Contains(stderr, "unknown revision or path not in the working tree") {
		return true
	} else if err == nil {
		return false
	}

	panic(err.Error())
}

func compare_commit(c1 Commit, c2 Commit) int {
	v1 := c1.Timestamp
	v2 := c2.Timestamp
	if v1 == v2 {
		return 0
	}

	if v1 < v2 {
		return -1
	}

	return 1
}

func commits_insert_sorted_unique(commits []Commit, commit Commit) []Commit {
	idx := bin_search(commits, commit, compare_commit)

	if idx < 0 {
		return slices.Insert(commits, ^idx, commit)
	}

	return commits
}

func commit_diffs(commit Commit, repo Repo) []Diff {
	ret := []Diff{}

	var prev_commit string
	if commit.Root {
		prev_commit = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
	} else {
		prev_commit = "HEAD^"
	}

	stdout, _, err := run_git_sync(repo.Path, "diff", "--numstat", prev_commit, commit.Hash)
	check(err)

	diff_lines := strings.Split(stdout, "\n")

	for _, line := range diff_lines {
		split := strings.Fields(line)

		if len(split) != 3 {
			continue
		}

		added, err := strconv.ParseUint(split[0], 10, 32)
		check(err)
		removed, err := strconv.ParseUint(split[1], 10, 32)
		check(err)
		ret = append(ret, Diff{File: split[2], Added: uint(added), Removed: uint(removed)})
	}

	return ret
}
