package main

import (
	"fmt"
	"os"

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
	Repositories []string
	Emails       []string
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

var output_path string
var config Config

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
				output_path = os.Args[i+1]
				i++
			}
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
	check_field(len(config.Emails), "emails")
	check_field(len(config.Authors), "authors")

	err = os.MkdirAll(config.Location, os.FileMode(0777))
	check(err)

	for _, id := range config.Repositories {
		repo := repo_new(id)
		repo_refresh(repo)

		counts := repo_check(repo)
		for k, v := range counts {
			fmt.Println(k, "value is", v)
		}
	}
}
