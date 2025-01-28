package main

import (
	"cmp"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"
	"text/template"

	"github.com/go-enry/go-enry/v2"
)

type langLinePair struct {
	lang  string
	lines int
}

var svgTmplFuncMap template.FuncMap
var entryTmplFuncMap template.FuncMap

func fmt_int(n int) string {
	numStr := strconv.Itoa(n)
	for i := len(numStr) - 3; i > 0; i -= 3 {
		numStr = numStr[:i] + "," + numStr[i:]
	}

	return numStr
}

func indent(s string, by int) string {
	if len(s) == 0 {
		return ""
	}

	split := strings.Split(s, "\n")
	builder := new(strings.Builder)

	for _, str := range split {
		builder.WriteString(strings.Repeat("\t", by))
		builder.WriteString(str)
		builder.WriteByte('\n')
	}

	return builder.String()
}

func create_svg(langs map[string]int) {
	svgTmplFuncMap = template.FuncMap{
		"indent": indent,
	}

	entryTmplFuncMap = template.FuncMap{
		"div": func(n1 int, n2 int) int {
			return n1 / n2
		},
		"sub": func(n1 int, n2 int) int {
			return n1 - n2
		},
	}

	langsSorted := []langLinePair{}

	for k, v := range langs {
		if k == "Unknown" || k == "Text" || k == "Markdown" || slices.Contains(config.Ignore.Langs, k) {
			delete(langs, k)
			continue
		}

		lp := langLinePair{k, v}
		pos := bin_search(langsSorted, lp, func(lp1 langLinePair, lp2 langLinePair) int {
			return cmp.Compare(lp1.lines, lp2.lines)
		})

		if pos < 0 {
			langsSorted = slices.Insert(langsSorted, ^pos, lp)
		}
	}

	langsLen := len(langs)
	keep := min(langsLen, config.LangsCount)
	langsSorted = langsSorted[:keep]

	totalLines := 0
	for _, v := range langsSorted {
		totalLines += v.lines
	}

	outputFile, err := os.Create(outputPath)
	check(err)
	defer outputFile.Close()

	switch strings.ToLower(config.Style.Type) {
	case "vertical":
		create_vertical(float64(totalLines), langsSorted, outputFile)
	case "compact":
		create_compact(float64(totalLines), langsSorted, outputFile)
	default:
		panic(fmt.Sprintf("Unknown style %s", config.Style))
	}
}

type SVGTheme struct {
	CardTitle string
	CardBG    string
}

type SVGData struct {
	Width   int
	Height  int
	TitleX  int
	Entries string
	Styles  string
	Theme   SVGTheme
}

const SVGTEMPLATESTRING = `<svg width="{{ .Width }}" height="{{ .Height }}" viewBox="0 0 {{ .Width }} {{ .Height }}" fill="none" xmlns="http://www.w3.org/2000/svg" role="img"
	aria-labelledby="descId">
	<title id="titleId"></title>
	<desc id="descId"></desc>
	<style>
{{ indent .Styles 2 }}
		.header {
			font: 600 18px 'Segoe UI', Ubuntu, Sans-Serif;
			fill: #70a5fd;
			animation: fadeInAnimation 0.8s ease-in-out forwards;
		}

		.rectbg {
			fill: #ddd;
		}

		@supports(-moz-appearance: auto) {

			/* Selector detects Firefox */
			.header {
				font-size: 15.5px;
			}
		}

		@keyframes slideInAnimation {
			from {
				width: 0;
			}

			to {
				width: calc(100%-100px);
			}
		}

		@keyframes growWidthAnimation {
			from {
				width: 0;
			}

			to {
				width: 100%;
			}
		}

		.stat {
			font: 600 14px 'Segoe UI', Ubuntu, "Helvetica Neue", Sans-Serif;
			fill: #38bdae;
		}

		@supports(-moz-appearance: auto) {

			/* Selector detects Firefox */
			.stat {
				font-size: 12px;
			}
		}

		.bold {
			font-weight: 700
		}

		.lang-name {
			font: 400 11px "Segoe UI", Ubuntu, Sans-Serif;
			fill: #38bdae;
		}

		.stagger {
			opacity: 0;
			animation: fadeInAnimation 0.3s ease-in-out forwards;
		}

		#rect-mask rect {
			animation: slideInAnimation 1s ease-in-out forwards;
		}

		.lang-progress {
			animation: growWidthAnimation 0.6s ease-in-out forwards;
		}

		/* Animations */
		@keyframes scaleInAnimation {
			from {
				transform: translate(-5px, 5px) scale(0);
			}

			to {
				transform: translate(-5px, 5px) scale(1);
			}
		}

		@keyframes fadeInAnimation {
			from {
				opacity: 0;
			}

			to {
				opacity: 1;
			}
		}
	</style>

	<rect data-testid="card-bg" x="0.5" y="0.5" rx="4.5" height="100%" stroke="#e4e2e2" width="100%" fill="#1a1b27"
		stroke-opacity="0" />

	<g data-testid="card-title" transform="translate(0, 35)">
		<g transform="translate(0, 0)">
			<text x="{{ .TitleX }}" y="0" class="header" data-testid="header">Most Used Languages</text>
		</g>
	</g>

	<g data-testid="main-card-body" transform="translate(0, 55)">
		<svg data-testid="lang-items">
{{ indent .Entries 3 }}
		</svg>
	</g>
</svg>
`

type EntryData struct {
	LangName   string
	TotalWidth int
	XOffset    int
	YOffset    int
	CountStr   string
	PercStr    string
	Delay      int
	FillDelay  int
	RectW      int
	Color      string
}

func process_entries(tmpl *template.Template, data []EntryData) string {
	builder := new(strings.Builder)
	dlen := len(data)

	for i, entry := range data {
		err := tmpl.Execute(builder, entry)
		check(err)

		if i != dlen-1 {
			builder.WriteByte('\n')
		}
	}

	return builder.String()
}

func process_template(data SVGData, writer io.Writer) {
	tmpl, err := template.New("svg").Funcs(svgTmplFuncMap).Parse(SVGTEMPLATESTRING)
	check(err)

	err = tmpl.Execute(writer, data)
	check(err)
}

func create_compact(totalLines float64, langsSorted []langLinePair, outputFile *os.File) {
	const MASK = `<mask id="rect-mask">
	<rect x="0" y="0" width="%d" height="8" fill="white" rx="5" />
</mask>` + "\n"

	const SVGENTRY = `<g transform="translate({{ .XOffset }}, {{ .YOffset }})">
	<g class="stagger" style="animation-delay: {{ .Delay }}ms">
		<circle r="4" cx="{{ if eq .XOffset 0 }} 31 {{ else }} 15 {{ end }}" cy="14" fill="{{ .Color }}" />
		<text data-testid="lang-name" x="{{ if eq .XOffset 0 }} 51 {{ else }} 29 {{ end }}" y="15" class="lang-name">{{ .LangName }}</text>
		{{ if eq .CountStr "" }} {{ else }} <text x="{{ if eq .XOffset 0 }} {{ $half := div .TotalWidth 2 }}{{ sub $half 60 }} {{ else }} {{ $half := div .TotalWidth 2 }}{{ sub $half 76 }} {{ end }}" y="15" class="lang-name lang-perc">{{ .CountStr }}</text> {{ end }}
		<text x="{{ if eq .XOffset 0 }} {{ $half := div .TotalWidth 2 }}{{ sub $half 15 }} {{ else }} {{ $half := div .TotalWidth 2 }}{{ sub $half 31 }} {{ end }}" y="15" class="lang-name lang-perc">{{ .PercStr }}%</text>
	</g>
</g>`

	tmpl, err := template.New("entry").Funcs(entryTmplFuncMap).Parse(SVGENTRY)
	check(err)

	var width int

	if config.Style.Count == "none" {
		width = 340
	} else {
		width = 480
	}

	count := len(langsSorted)
	entries := make([]EntryData, count)

	for i, lp := range langsSorted {
		perc := float64(lp.lines) / totalLines
		percStr := strings.TrimRight(fmt.Sprintf("%.2f", perc*100), "0")
		percStrLen := len(percStr)

		if percStr[percStrLen-1] == '.' {
			percStr = percStr[:percStrLen-1]
		}

		countStr := ""
		if config.Style.Count == "lines" {
			countStr = fmt.Sprintf("%s lines", fmt_int(lp.lines))
		}

		entries[i] = EntryData{
			LangName:   lp.lang,
			TotalWidth: width,
			XOffset:    i % 2 * (width / 2),
			YOffset:    i / 2 * 20,
			CountStr:   countStr,
			PercStr:    percStr,
			Delay:      450 + i*150,
			FillDelay:  750 + i*150,
			RectW:      max(int(perc*100), 2),
			Color:      enry.GetColor(lp.lang),
		}
	}

	process_template(SVGData{
		Width:   width,
		Height:  (count/2)*20 + 85,
		TitleX:  width / 2,
		Entries: MASK + process_entries(tmpl, entries),
		Styles: `.header { text-anchor: middle; }
.lang-name { dominant-baseline: middle }
.lang-perc { text-anchor: end; }`,
	}, outputFile)
}

func create_vertical(totalLines float64, langsSorted []langLinePair, outputFile *os.File) {
	const SVGENTRY = `<g transform="translate({{ .XOffset }}, {{ .YOffset }})">
	<g class="stagger" style="animation-delay: {{ .Delay }}ms">
		<text data-testid="lang-name" x="2" y="15" class="lang-name">{{ .LangName }} {{ .CountStr }}</text>
		<text x="215" y="34" class="lang-name">{{ .PercStr }}%</text>
		<svg width="205" x="0" y="25">
			<rect class="rectbg" rx="5" ry="5" x="0" y="0" width="205" height="8"></rect>
			<svg data-testid="lang-progress" width="{{ .RectW }}%">
				<rect height="8" fill="{{ .Color }}" rx="5" ry="5" x="0" y="0" class="lang-progress"
					style="animation-delay: {{ .FillDelay }}ms;" />
			</svg>
		</svg>
	</g>
</g>`

	tmpl, err := template.New("entry").Parse(SVGENTRY)
	check(err)

	count := len(langsSorted)
	entries := make([]EntryData, count)

	for i, lp := range langsSorted {
		perc := float64(lp.lines) / totalLines
		percStr := strings.TrimRight(fmt.Sprintf("%.2f", perc*100), "0")
		percStrLen := len(percStr)

		if percStr[percStrLen-1] == '.' {
			percStr = percStr[:percStrLen-1]
		}

		countStr := ""
		if config.Style.Count == "lines" {
			countStr = fmt.Sprintf("(%s lines)", fmt_int(lp.lines))
		}

		entries[i] = EntryData{
			LangName:  lp.lang,
			XOffset:   25,
			YOffset:   i * 40,
			CountStr:  countStr,
			PercStr:   percStr,
			Delay:     450 + i*150,
			FillDelay: 750 + i*150,
			RectW:     max(int(perc*100), 2),
			Color:     enry.GetColor(lp.lang),
		}
	}

	process_template(SVGData{
		Width:   300,
		Height:  count*40 + 85,
		TitleX:  25,
		Entries: process_entries(tmpl, entries),
	}, outputFile)
}
