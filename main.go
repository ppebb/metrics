package main

import (
	"fmt"
	"os"
	"slices"

	"gopkg.in/yaml.v3"
)

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func check_field(l int, name string) {
	if l == 0 {
		panic(fmt.Sprintf("Config is missing field %s!", name))
	}
}

type Config struct {
	Location     string
	Indepth      bool
	Token        string
	ExcludeForks bool
	Users        []string
	Orgs         []string
	Repositories []string
	Authors      []string
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
var dryRun = false

func main() {
	var config_path string

	argsLen := len(os.Args)
	for i := 0; i < argsLen; i++ {
		arg := os.Args[i]
		switch arg {
		case "-c", "--config":
			if argsLen > i+1 {
				config_path = os.Args[i+1]
				i++
			}
		case "-o", "--output":
			if argsLen > i+1 {
				outputPath = os.Args[i+1]
				i++
			}
		case "-d", "--dry-run":
			dryRun = true
		}
	}

	if len(config_path) == 0 {
		panic("Missing config argument, provide a config.yml with -c or --config")
	}

	data, err := os.ReadFile(config_path)
	check(err)

	config = Config{}
	err = yaml.Unmarshal(data, &config)
	check(err)

	check_field(len(config.Location), "location")
	check_field(len(config.Repositories), "repositories")
	check_field(len(config.Authors), "authors")

	err = os.MkdirAll(config.Location, os.FileMode(0777))
	check(err)

	reposToCheck := config.Repositories

	copyToReposToCheck := func(repoResponses []RepoResponse) {
		for _, repo := range repoResponses {
			if config.ExcludeForks && repo.Fork {
				continue
			}

			if !slices.Contains(reposToCheck, repo.Full_Name) {
				reposToCheck = append(reposToCheck, repo.Full_Name)
			}
		}
	}

	for _, user := range config.Users {
		copyToReposToCheck(github_get_account_repos(user, false, config.Token))
	}

	for _, org := range config.Orgs {
		copyToReposToCheck(github_get_account_repos(org, true, config.Token))
	}

	if dryRun {
		fmt.Println("The following repositories will be cloned and analyzed:")
		for _, v := range reposToCheck {
			fmt.Printf("    %s\n", v)
		}

		return
	}

	cumulativeLangs := map[string]int{}

	for _, id := range reposToCheck {
		repo := repo_new(id)

		var counts map[string]int
		if config.Indepth {
			counts = repo_count_by_commit(&repo)
		} else {
			counts = repo_count(&repo)
		}

		for k, v := range counts {
			cumulativeLangs[k] += v
		}
	}

	for k, v := range cumulativeLangs {
		fmt.Println(k, v)
	}
}
