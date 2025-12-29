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
	mu    sync.Mutex
	v     map[string]*LineBytePair
	l     map[string][]LineBytePairForLang
	f     int
	repos []Repo
}

func printHelp() {
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
	var configPath string
	var dryRun = false
	var silent = false

	argsLen := len(os.Args)

	if argsLen <= 1 {
		fmt.Println("No arguments provided! --config is required to continue.")
		printHelp()
	}

	for i := 1; i < argsLen; i++ {
		arg := os.Args[i]
		switch arg {
		case "-h", "--help":
			printHelp()
		case "-c", "--config":
			if argsLen > i+1 {
				configPath = os.Args[i+1]
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
			printHelp()
		}
	}

	if len(configPath) == 0 {
		panic("Missing config argument, provide a config.yml with -c or --config")
	}

	if len(outputPath) == 0 {
		fmt.Println("No output path specified! Defaulting to ./langs.svg")
		outputPath = "./langs.svg"
	}

	initLog(silent)
	defer logClose()
	initConfig(configPath)

	if dryRun {
		fmt.Println("The following repositories will be cloned and analyzed:")
		for _, v := range reposToCheck {
			fmt.Printf("    %s\n", v)
		}

		logResetCursor()
		return
	}

	cursorY = logGetCursorPos()

	cumulative := ConcData{
		mu:    sync.Mutex{},
		v:     map[string]*LineBytePair{},
		l:     map[string][]LineBytePairForLang{},
		f:     0,
		repos: []Repo{},
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
					lastRepo.checkoutBranch(lastRepo.LatestBranch)
				}

				pstr := strings.ReplaceAll(fmt.Sprint(r), "\n", "")
				logProgess(lastRepo, fmt.Sprintf("Panic caught, %s, exiting...", pstr), -1)
				closeOnce()
			}
		}()

	REPOSLOOP:
		for id := range repos {
			select {
			case _, ok := <-cancel:
				if !ok {
					logProgess(lastRepo, "Exited", -1)
					break REPOSLOOP
				}
			default:
				log(Info, nil, fmt.Sprintf("WorkerID %d: preparing to initialize repo %s", workerID, id))
				repo := Repo{
					Identifier: id,
				}

				lastRepo = &repo
				initRepo(&repo)

				if len(repo.LatestCommit.Hash) == 0 {
					continue
				}

				var counts map[string]*LineBytePair
				if config.Indepth {
					counts = repo.countByCommit()
				} else {
					counts = repo.count()
				}

				cumulative.mu.Lock()
				for k, v := range counts {
					if cumulative.v[k] == nil {
						cumulative.v[k] = &LineBytePair{}
					}

					cumulative.v[k].lines += v.lines
					cumulative.v[k].bytes += v.bytes

					if cumulative.l[k] == nil {
						cumulative.l[k] = []LineBytePairForLang{}
					}

					cumulative.l[k] = append(cumulative.l[k], LineBytePairForLang{
						lang:  repo.Identifier,
						lines: v.lines,
						bytes: v.bytes,
					})

				}

				cumulative.f += len(repo.UniqueFiles)
				cumulative.repos = append(cumulative.repos, repo)
				cumulative.mu.Unlock()
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

	logResetTermIfNeeded()

	if closed {
		os.Exit(1)
	}

	createSVG(cumulative.v, cumulative.f)

	for k, v := range cumulative.l {
		totals := cumulative.v[k]
		lines := 0
		bytes := 0

		if totals != nil {
			lines = totals.lines
			bytes = totals.bytes
		}

		var msg strings.Builder
		fmt.Fprintf(&msg, "Language %s: %d lines, %d bytes\n", k, lines, bytes)

		for _, triplet := range v {
			fmt.Fprintf(&msg, "ID: %s, Lines: %d, Bytes: %d\n", triplet.lang, triplet.lines, triplet.bytes)
		}

		log(Info, nil, msg.String())
	}

	for _, repo := range cumulative.repos {
		var msg strings.Builder
		fmt.Fprintf(&msg, "Commit-by-commit stats for repo '%s'\n", repo.Identifier)

		for _, hash := range repo.CommitHashesOrdered {
			v := repo.CommitCounts[hash]
			fmt.Fprintf(&msg, "Commit: %s, Lines: %d, Bytes: %d\n", hash, v.lines, v.bytes)
		}

		log(Info, nil, msg.String())
	}
}
