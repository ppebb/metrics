package main

import (
	"fmt"
	"os"
	"regexp"
	"slices"

	"gopkg.in/yaml.v3"
)

func check_empty[T string | []string](t T, name string) {
	if len(t) == 0 {
		panic(fmt.Sprintf("Config is missing field %s!", name))
	}
}

type Config struct {
	Location   string
	Indepth    bool
	CountTotal bool
	LangsCount int
	Style      struct {
		Type      string
		Count     string
		BytesBase int
	}
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
		Langs         []string
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

	check_empty(config.Location, "location")
	check_empty(config.Repositories, "repositories")
	check_empty(config.Authors, "authors")
	check_empty(config.Style.Type, "style.type")
	check_empty(config.Style.Count, "style.count")
	check_empty(config.Token, "token")

	if config.Style.Count == "bytes" && config.Style.BytesBase != 1000 && config.Style.BytesBase != 1024 {
		panic("config.style.bytesbase must be either 1000 or 1024!")
	}

	if config.Parallel == 0 {
		config.Parallel = 1
	}

	if config.LangsCount == 0 {
		config.LangsCount = 5
	}

	err = os.MkdirAll(config.Location, os.FileMode(0777))
	check(err)

	reposToCheck = config.Repositories

	var testRepo func(repo string) (bool, string)

	if len(config.Filters) > 0 {
		filters := []*regexp.Regexp{}

		for _, pattern := range config.Filters {
			regex, err := regexp.Compile(pattern)

			if err != nil {
				panic(fmt.Sprintf("Pattern \"%s\" failed to compile to regex: error %s", pattern, err.Error()))
			}

			filters = append(filters, regex)
		}

		testRepo = func(repo string) (bool, string) {
			for _, regex := range filters {
				if regex.MatchString(repo) {
					return true, regex.String()
				}
			}

			return false, ""
		}
	} else {
		testRepo = func(_ string) (bool, string) { return false, "" }
	}

	copyToReposToCheck := func(repoResponses []RepoResponse) {
		for _, repo := range repoResponses {
			if config.ExcludeForks && repo.Fork {
				log(Info, nil, fmt.Sprintf("Skipping forked repository %s", repo.Full_Name))
				continue
			}

			if matched, pat := testRepo(repo.Full_Name); matched {
				log(Info, nil, fmt.Sprintf("Skipping repository %s, matched %s", repo.Full_Name, pat))
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
