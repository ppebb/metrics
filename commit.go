package main

import "slices"

type Commit struct {
	Hash      string
	Timestamp uint64
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
