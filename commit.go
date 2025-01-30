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

func commit_new(repo *Repo, hash string, timestamp uint64) Commit {
	return Commit{
		Hash:      hash,
		Timestamp: timestamp,
		Root:      commit_is_root(repo, hash),
	}
}

func commit_is_root(repo *Repo, hash string) bool {
	_, stderr, err := run_git_sync(repo.Path, "rev-parse", hash+"^")

	if strings.Contains(stderr, "unknown revision or path not in the working tree") {
		return true
	} else if err == nil {
		return false
	}

	panic(err.Error())
}

func compare_commit(c1 Commit, c2 Commit) int {
	return cmp.Compare(c1.Timestamp, c2.Timestamp)
}

func commits_insert_sorted_unique(commits []Commit, commit Commit) []Commit {
	idx := bin_search(commits, commit, compare_commit)

	if idx < 0 {
		return slices.Insert(commits, ^idx, commit)
	}

	return commits
}

func get_bytes_for_file_hash(hash string, repo *Repo) (int, error) {
	stdout, _, err := run_git_sync(repo.Path, "cat-file", "-s", hash)
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

func (commit Commit) get_diffs(repo *Repo) []Diff {
	ret := []Diff{}

	var prev_commit string
	if commit.Root {
		prev_commit = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
	} else {
		prev_commit = "HEAD^"
	}

	stdout, _, err := run_git_sync(repo.Path, "diff", "--patch", prev_commit, commit.Hash)
	check(err)

	diff_lines := strings.Split(stdout, "\n")

	var currentDiff Diff

	for _, line := range diff_lines {
		// Minimum viable line is a +/- followed by any other character
		if len(line) < 2 {
			continue
		}

		if str_starts_with(line, "diff") {
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
				Added:   LineBytePair{},
				Removed: LineBytePair{},
			}

			continue
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
