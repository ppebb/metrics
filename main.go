package main

import (
	"fmt"
	"os"
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

func print_help() {
	fmt.Printf(`
ppeb's git language metrics generator!!!

Usage: ./ppebtrics [OPTIONS]
 -h|--help             Display this message and exit
 -c|--config           Specify the path to your config.yml
 -o|--output           Specify the output path of your svg
 -d|--dry-run          Dry run! List the repos to be cloned and analyzed
`)

	os.Exit(1)
}

func main() {
	var config_path string
	var dryRun = false

	argsLen := len(os.Args)
	for i := 1; i < argsLen; i++ {
		arg := os.Args[i]
		switch arg {
		case "-h", "--help":
			print_help()
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
		default:
			fmt.Printf("Unknown argument %s!\n", arg)
			print_help()
		}
	}

	if len(config_path) == 0 {
		panic("Missing config argument, provide a config.yml with -c or --config")
	}

	config_init(config_path)

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
