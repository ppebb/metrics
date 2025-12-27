package main

import (
	"fmt"
	"os"
	"regexp"
	"slices"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

func checkEmpty[T string | []string](t T, name string) {
	if len(t) == 0 {
		panic(fmt.Sprintf("Config is missing field %s!", name))
	}
}

type SVGTheme struct {
	CardBG     string
	CardStroke string
	Header     string
	SubHeader  string
	RectBg     string
	LangName   string
	Count      string
	Percent    string
}

type Config struct {
	Location    string
	Indepth     bool
	CountTotal  bool
	CountSpaces bool
	LangsCount  int
	Style       struct {
		Theme     string
		Type      string
		Count     string
		BytesBase int
		ShowTotal bool
	}
	Token        string
	ExcludeForks bool
	Parallel     uint8
	Users        []string
	Orgs         []string
	Repositories []string
	Authors      []string
	Filters      []string
	Commits      []string
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
var theme SVGTheme
var reposToCheck []string

func initConfig(path string) {
	data, err := os.ReadFile(path)
	check(err)

	config = Config{}
	err = yaml.Unmarshal(data, &config)
	check(err)

	checkEmpty(config.Location, "location")
	// check_empty(config.Repositories, "repositories")
	checkEmpty(config.Authors, "authors")
	checkEmpty(config.Style.Type, "style.type")
	checkEmpty(config.Style.Count, "style.count")
	checkEmpty(config.Style.Theme, "style.theme")
	// check_empty(config.Token, "token")

	if !fileExists(config.Style.Theme) {
		panic(fmt.Sprintf("config.style.theme (%s) does not exist on disk!", config.Style.Theme))
	}

	data, err = os.ReadFile(config.Style.Theme)
	check(err)

	theme = SVGTheme{}
	err = yaml.Unmarshal(data, &theme)
	check(err)

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
				logEcho(Info, nil, fmt.Sprintf("Skipping forked repository %s", repo.Full_Name), true)
				continue
			}

			if matched, pat := testRepo(repo.Full_Name); matched {
				logEcho(Info, nil, fmt.Sprintf("Skipping repository %s, matched filter %s", repo.Full_Name, pat), true)
				continue
			}

			if !slices.Contains(reposToCheck, repo.Full_Name) {
				reposToCheck = append(reposToCheck, repo.Full_Name)
			}
		}
	}

	for _, user := range config.Users {
		logEcho(Info, nil, fmt.Sprintf("Fetching repositories for user %s", user), true)
		copyToReposToCheck(githubGetAccountRepos(user, false, config.Token))
	}

	for _, org := range config.Orgs {
		logEcho(Info, nil, fmt.Sprintf("Fetching repositories for org %s", org), true)
		copyToReposToCheck(githubGetAccountRepos(org, true, config.Token))
	}

	if len(reposToCheck) == 0 {
		panic("There are no repositorites to check! Either all have been filtered or none were provided. See config.users, config.orgs, and config.repositorites")
	}

	sort.Slice(reposToCheck, func(i, j int) bool {
		return strings.ToLower(reposToCheck[i]) < strings.ToLower(reposToCheck[j])
	})
}
