package main

import (
	"fmt"
	"os"
	"regexp"
	"slices"

	"gopkg.in/yaml.v3"
)

func check_field(l int, name string) {
	if l == 0 {
		panic(fmt.Sprintf("Config is missing field %s!", name))
	}
}

type Config struct {
	Location     string
	Indepth      bool
	Countloc     bool
	Token        string
	ExcludeForks bool
	Parallel     uint8
	Users        []string
	Orgs         []string
	Repositories []string
	Authors      []string
	Filters      []string
	Ignore       struct {
		Vendor        bool
		Dotfiles      bool
		Configuration bool
		Image         bool
		Test          bool
		Binary        bool
		Generated     bool
	}
}

var outputPath string
var config Config
var reposToCheck []string

func config_init(path string) {
	data, err := os.ReadFile(path)
	check(err)

	config = Config{}
	err = yaml.Unmarshal(data, &config)
	check(err)

	check_field(len(config.Location), "location")
	check_field(len(config.Repositories), "repositories")
	check_field(len(config.Authors), "authors")

	err = os.MkdirAll(config.Location, os.FileMode(0777))
	check(err)

	reposToCheck = config.Repositories

	var testRepo func(repo string) bool

	if len(config.Filters) > 0 {
		filters := []*regexp.Regexp{}

		for _, pattern := range config.Filters {
			regex, err := regexp.Compile(pattern)

			if err != nil {
				panic(fmt.Sprintf("Pattern \"%s\" failed to compile to regex: error %s", pattern, err.Error()))
			}

			filters = append(filters, regex)
		}

		testRepo = func(repo string) bool {
			for _, regex := range filters {
				if regex.MatchString(repo) {
					return true
				}
			}

			return false
		}
	} else {
		testRepo = func(_ string) bool { return false }
	}

	copyToReposToCheck := func(repoResponses []RepoResponse) {
		for _, repo := range repoResponses {
			if config.ExcludeForks && repo.Fork {
				continue
			}

			if testRepo(repo.Full_Name) {
				continue
			}

			if !slices.Contains(reposToCheck, repo.Full_Name) {
				reposToCheck = append(reposToCheck, repo.Full_Name)
			}
		}
	}

	for _, user := range config.Users {
		fmt.Printf("Fetching repositories for user %s\n", user)
		copyToReposToCheck(github_get_account_repos(user, false, config.Token))
	}

	for _, org := range config.Orgs {
		fmt.Printf("Fetching repositories for org %s\n", org)
		copyToReposToCheck(github_get_account_repos(org, true, config.Token))
	}
}
