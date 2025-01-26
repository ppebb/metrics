package main

import (
	"fmt"
	"os"
	"runtime/debug"
	"strings"
	"sync"
)

func check(e error) {
	if e != nil {
		panic(e)
	}
}

type ConcStringIntMap struct {
	mu sync.Mutex
	v  map[string]int
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

	log_init()
	defer log_close()
	config_init(config_path)
	log_populate_cursor_pos()

	if dryRun {
		fmt.Println("The following repositories will be cloned and analyzed:")
		for _, v := range reposToCheck {
			fmt.Printf("    %s\n", v)
		}

		return
	}

	cumulativeLangs := ConcStringIntMap{
		mu: sync.Mutex{},
		v:  map[string]int{},
	}

	var cancelChannel chan bool
	closed := false
	closeOnce := sync.OnceFunc(func() { close(cancelChannel); closed = true })

	countRepo := func(repos <-chan string, cancel <-chan bool, wg *sync.WaitGroup) {
		defer wg.Done()

		var lastRepo *Repo

		defer func() {
			if r := recover(); r != nil {
				log(Critical, lastRepo, fmt.Sprintf("Panic caught, %s, exiting...\n%s", r, debug.Stack()))

				if lastRepo != nil {
					log(Info, lastRepo, fmt.Sprintf("Reverting to branch %s", lastRepo.LatestBranch))
					lastRepo.checkout_branch(lastRepo.LatestBranch)
				}

				pstr := strings.Replace(fmt.Sprint(r), "\n", "", -1)
				log_progress(lastRepo, fmt.Sprintf("Panic caught, %s, exiting...", pstr), -1)
				closeOnce()
			}
		}()

	REPOSLOOP:
		for id := range repos {
			select {
			case _, ok := <-cancel:
				if !ok {
					log_progress(lastRepo, "Exited", -1)
					break REPOSLOOP
				}
			default:
				repo := repo_new(id)
				lastRepo = &repo

				var counts map[string]int
				if config.Indepth {
					counts = repo.repo_count_by_commit()
				} else {
					counts = repo.repo_count()
				}

				cumulativeLangs.mu.Lock()
				for k, v := range counts {
					cumulativeLangs.v[k] += v
				}
				cumulativeLangs.mu.Unlock()
			}
		}
	}

	repoChannel := make(chan string, len(reposToCheck))
	cancelChannel = make(chan bool)
	var wg sync.WaitGroup

	for i := 0; i < int(config.Parallel); i++ {
		wg.Add(1)
		go countRepo(repoChannel, cancelChannel, &wg)
	}

	for _, id := range reposToCheck {
		repoChannel <- id
	}

	close(repoChannel)
	wg.Wait()

	log_reset_term_if_needed()

	if closed {
		return
	}

	for k, v := range cumulativeLangs.v {
		fmt.Println(k, v)
	}
}
