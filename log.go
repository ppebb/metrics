package main

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

type fdSafe struct {
	mu sync.Mutex
	fd *os.File
}

var progMu sync.Mutex
var counter int
var fd fdSafe
var isTerminal bool
var termWidth int
var termHeight int
var cursorY int
var silent bool

func initLog(isSilent bool) {
	progMu = sync.Mutex{}
	counter = -1
	silent = isSilent

	dt := time.Now()

	err := os.MkdirAll("./logs", os.FileMode(0755))
	check(err)

	logFd, err := os.Create(fmt.Sprintf("./logs/%s.log", dt.Format(time.RFC822)))
	check(err)

	fd = fdSafe{
		mu: sync.Mutex{},
		fd: logFd,
	}

	if silent {
		return
	}

	o, err := os.Stdout.Stat()
	check(err)

	isTerminal = o.Mode()&os.ModeCharDevice == os.ModeCharDevice

	if isTerminal {
		ws, err := unix.IoctlGetWinsize(0, unix.TIOCGWINSZ)
		check(err)

		termWidth = int(ws.Col)
		termHeight = int(ws.Row)
		fmt.Print("\x1b[?25l")
	}
}

func logGetCursorPos() int {
	if !isTerminal {
		return -1
	}

	tIOS, err := unix.IoctlGetTermios(0, unix.TCGETS)
	check(err)

	tIOSOrig := *tIOS

	tIOS.Lflag &^= unix.ECHO | unix.ECHONL | unix.ICANON | unix.ISIG | unix.IEXTEN
	unix.IoctlSetTermios(0, unix.TCSETS, tIOS)
	defer unix.IoctlSetTermios(0, unix.TCSETS, &tIOSOrig)

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("\x1b[6n")

	text, err := reader.ReadSlice('R')

	check(err)

	// parse the row and column
	if strings.Contains(string(text), ";") {
		re := regexp.MustCompile(`\d+;\d+`)
		line := re.FindString(string(text))
		row, err := strconv.Atoi(strings.Split(line, ";")[0])
		check(err)

		return row - 1
	} else {
		panic("Unable to determine cursor position")
	}
}

func logResetCursor() {
	if isTerminal {
		fmt.Print("\x1b[?25h")
	}
}

func logResetTermIfNeeded() {
	if isTerminal {
		fmt.Printf("\x1b[%d;%dH\n", min(termHeight, cursorY), termWidth)
		logResetCursor()
	}
}

func logClose() {
	fd.fd.Close()
}

type LogLevel uint8

const (
	Critical LogLevel = iota
	Warning
	Debug
	Info
)

func (e LogLevel) String() string {
	switch e {
	case Critical:
		return "Critical"
	case Warning:
		return "Warning"
	case Debug:
		return "Debug"
	case Info:
		return "Info"
	default:
		return fmt.Sprintf("%d", int(e))
	}
}

func logEcho(level LogLevel, repo *Repo, message string, echo bool) {
	fd.mu.Lock()
	defer fd.mu.Unlock()

	var msg string
	if repo != nil {
		msg = fmt.Sprintf("[%s] %s > %s\n", level, repo.Identifier, message)
	} else {
		msg = fmt.Sprintf("[%s] %s\n", level, message)
	}

	fd.fd.WriteString(msg)

	if echo && !silent {
		fmt.Print(msg)
	}
}

func log(level LogLevel, repo *Repo, message string) {
	logEcho(level, repo, message, false)
}

func logProgess(repo *Repo, message string, completion float64) {
	if !isTerminal || silent {
		return
	}

	progMu.Lock()
	defer progMu.Unlock()

	if repo.LogID == -1 {
		counter++
		repo.LogID = counter
		cursorY++
		if cursorY > termHeight && counter != 0 {
			fmt.Printf("\x1b[%d;%dH\n", min(termHeight, cursorY), termWidth)
		}
	}

	line := min(termHeight, cursorY) - (counter - repo.LogID)
	message = strings.ReplaceAll(message, "\n", "")
	message = strings.ReplaceAll(message, "\t", " ")
	front := fmt.Sprintf("%s > %s", repo.Identifier, message)
	progWidth := math.Floor(1.0 / 3.0 * float64(termWidth))
	lenHash := int(math.Floor(progWidth * completion))

	var prog string
	var perc string
	if completion >= 0 {
		prog = fmt.Sprintf(
			"[%s%s]",
			strings.Repeat("#", lenHash),
			strings.Repeat("-", int(progWidth)-lenHash),
		)

		perc = fmt.Sprintf("%d%%", int(completion*100))
		perc = fmt.Sprintf("%s%s", strings.Repeat(" ", 5-len(perc)), perc)
	} else {
		prog = fmt.Sprintf("[%s]", strings.Repeat("X", int(progWidth)))
		perc = " Err%"
	}

	spaceLen := termWidth - len(front) - len(prog) - len(perc)
	if spaceLen <= 0 {
		front = fmt.Sprintf("%s... ", front[:(len(front)+spaceLen)-4])
		spaceLen = 0
	}
	space := strings.Repeat(" ", spaceLen)
	fmt.Printf("\x1b[%d;0H%s%s%s%s", line, front, space, prog, perc)
}
