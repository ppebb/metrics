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

type ConcData struct {
	mu sync.Mutex
	v  map[string]*IntIntPair
	l  map[string][]StringIntIntTriplet
}

func print_help() {
	fmt.Printf(`
ppeb's git language metrics generator!!!

Usage: ./ppebtrics [OPTIONS]
 -h|--help             Display this message and exit
 -c|--config           Specify the path to your config.yml
 -o|--output           Specify the output path of your svg
 -d|--dry-run          Dry run! List the repos to be cloned and analyzed
 -s|--silent           Don't output to stdout
`)

	os.Exit(1)
}

func main() {
	var config_path string
	var dryRun = false
	var silent = false

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
		case "-s", "--silent":
			silent = true
		default:
			fmt.Printf("Unknown argument %s!\n", arg)
			print_help()
		}
	}

	if len(config_path) == 0 {
		panic("Missing config argument, provide a config.yml with -c or --config")
	}

	if len(outputPath) == 0 {
		fmt.Println("No output path specified! Defaulting to ./langs.svg")
		outputPath = "./langs.svg"
	}

	log_init(silent)
	defer log_close()
	config_init(config_path)

	if dryRun {
		fmt.Println("The following repositories will be cloned and analyzed:")
		for _, v := range reposToCheck {
			fmt.Printf("    %s\n", v)
		}

		return
	}

	cursorY = log_get_cursor_pos()

	cumulativeLangs := ConcData{
		mu: sync.Mutex{},
		v:  map[string]*IntIntPair{},
		l:  map[string][]StringIntIntTriplet{},
	}

	var cancelChannel chan bool
	closed := false
	closeOnce := sync.OnceFunc(func() { close(cancelChannel); closed = true })

	countRepo := func(workerID int, repos <-chan string, cancel <-chan bool, wg *sync.WaitGroup) {
		defer wg.Done()

		var lastRepo *Repo

		defer func() {
			if r := recover(); r != nil {
				log(Critical, lastRepo, fmt.Sprintf("Panic caught in WorkerID %d: %s, exiting...\n%s", workerID, r, debug.Stack()))

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
				log(Info, nil, fmt.Sprintf("WorkerID %d: preparing to initialize repo %s", workerID, id))
				repo := Repo{
					Identifier: id,
				}

				lastRepo = &repo
				repo_initialize(&repo)

				if len(repo.LatestCommit.Hash) == 0 {
					continue
				}

				var counts map[string]*IntIntPair
				if config.Indepth {
					counts = repo.repo_count_by_commit()
				} else {
					counts = repo.repo_count()
				}

				cumulativeLangs.mu.Lock()
				for k, v := range counts {
					if cumulativeLangs.v[k] == nil {
						cumulativeLangs.v[k] = &IntIntPair{}
					}

					cumulativeLangs.v[k].lines += v.lines
					cumulativeLangs.v[k].bytes += v.bytes

					if cumulativeLangs.l[k] == nil {
						cumulativeLangs.l[k] = []StringIntIntTriplet{}
					}

					cumulativeLangs.l[k] = append(cumulativeLangs.l[k], StringIntIntTriplet{
						lang:  repo.Identifier,
						lines: v.lines,
						bytes: v.bytes,
					})
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
		go countRepo(i, repoChannel, cancelChannel, &wg)
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

	create_svg(cumulativeLangs.v)

	for k, v := range cumulativeLangs.l {
		totals := cumulativeLangs.v[k]
		lines := 0
		bytes := 0

		if totals != nil {
			lines = totals.lines
			bytes = totals.bytes
		}
		msg := fmt.Sprintf("Language %s: %d lines, %d bytes\n", k, lines, bytes)

		for _, triplet := range v {
			msg += fmt.Sprintf("ID: %s, Lines: %d, Bytes: %d\n", triplet.lang, triplet.lines, triplet.bytes)
		}

		log(Info, nil, msg)
	}
}
