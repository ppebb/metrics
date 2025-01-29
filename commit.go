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

// TODO: Calculate diffs line by line so an actual added and removed can be calculated
func (commit Commit) get_diffs_bytes(repo *Repo) []Diff {
	ret := []Diff{}

	var prev_commit string
	if commit.Root {
		prev_commit = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
	} else {
		prev_commit = "HEAD^"
	}

	stdout, _, err := run_git_sync(repo.Path, "diff-tree", "-r", prev_commit, commit.Hash)
	check(err)

	diff_lines := strings.Split(stdout, "\n")

	log(Debug, repo, fmt.Sprintf("Calculating diffs for commit %s", commit.Hash))

	for _, line := range diff_lines {
		split := strings.Fields(line)

		if len(split) != 6 {
			continue
		}

		hashOld := split[2]
		hashNew := split[3]
		mode := split[4]
		fileName := split[5]

		log(Debug, repo, fmt.Sprintf("Calculating diff for commit %s file %s", commit.Hash, fileName))
		var bytes int

		switch mode {
		case "M":
			bytesOld, err := get_bytes_for_file_hash(hashOld, repo)
			check(err)

			bytesNew, err := get_bytes_for_file_hash(hashNew, repo)
			check(err)

			bytes = bytesNew - bytesOld
		case "A":
			bytes, err = get_bytes_for_file_hash(hashNew, repo)
			check(err)
		case "D":
			bytesOld, err := get_bytes_for_file_hash(hashOld, repo)
			check(err)

			bytes = -bytesOld
		default:
			log(Warning, repo, fmt.Sprintf("Unhandled mode %s in commit %s file %s", mode, commit.Hash, fileName))
			continue
		}

		ret = append(ret, Diff{File: fileName, Added: uint(iabs(bytes)), Removed: 0})
	}

	return ret
}

func (commit Commit) get_diffs_lines(repo *Repo) []Diff {
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

		if split[0] == "-" || split[1] == "-" {
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
