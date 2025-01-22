package main

import (
	"slices"
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
