package main

import (
	"cmp"
	"fmt"
	"io"
	"math"
	"os"
	"slices"
	"strconv"
	"strings"
	"text/template"

	"github.com/go-enry/go-enry/v2"
)

type LangLineByteTriplet struct {
	lang  string
	lines int
	bytes int
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

func fmt_double(n float64) string {
	return strings.TrimRight(fmt.Sprintf("%.2f", n), "0")
}

func fmt_bytes(n int, base int) string {
	var prefix []string

	prefix_1000 := []string{"", "k", "M", "G", "T", "P", "E", "Z", "Y"}
	prefix_1024 := []string{"", "Ki", "Mi", "Gi", "Ti", "Pi", "Ei", "Zi", "Yi"}

	switch base {
	case 1000:
		prefix = prefix_1000
	case 1024:
		prefix = prefix_1024
	default:
		panic("wrong prefix")
	}

	fbase := float64(base)
	scaled := float64(n)
	j := 0
	for i := range prefix {
		j = i

		if scaled <= fbase {
			break
		}

		scaled /= fbase
	}

	return fmt.Sprintf("%s %s", fmt_double(scaled), prefix[j])
}

func fmt_count(lt LangLineByteTriplet) string {
	switch config.Style.Count {
	case "lines":
		return fmt.Sprintf("%d lines", lt.lines)
	case "bytes":
		return fmt.Sprintf("%sB", fmt_bytes(lt.bytes, config.Style.BytesBase))
	default:
		panic("Unknown config.style.count")
	}
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

func create_svg(langs map[string]*LineBytePair) {
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

	langsSorted := []LangLineByteTriplet{}

	for k, v := range langs {
		if k == "Unknown" || k == "Text" || k == "Markdown" || slices.Contains(config.Ignore.Langs, k) {
			delete(langs, k)
			continue
		}

		lt := LangLineByteTriplet{
			lang:  k,
			lines: v.lines,
			bytes: v.bytes,
		}
		pos := bin_search(langsSorted, lt, func(lp1 LangLineByteTriplet, lp2 LangLineByteTriplet) int {
			return cmp.Compare(lp1.lines, lp2.lines)
		})

		if pos < 0 {
			langsSorted = slices.Insert(langsSorted, ^pos, lt)
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
		panic(fmt.Sprintf("Unknown style %s", config.Style.Type))
	}
}

type SVGData struct {
	Width   int
	Height  int
	TitleX  int
	Entries string
	Styles  string
	Theme   SVGTheme
}

const SVGTEMPLATESTRING = `<svg width="{{ .Width }}" height="{{ .Height }}" viewBox="0 0 {{ .Width }}
		{{ .Height }}" fill="none" xmlns="http://www.w3.org/2000/svg" role="img"
	aria-labelledby="descId">
	<title id="titleId"></title>
	<desc id="descId"></desc>
	<style>
{{ indent .Styles 2 }}
		.header {
			font: 600 18px 'Segoe UI', Ubuntu, Sans-Serif;
			fill: {{ .Theme.Header }};
			animation: fadeInAnimation 0.8s ease-in-out forwards;
		}

		.rectbg {
			fill: {{ .Theme.RectBg }};
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

		.bold {
			font-weight: 700;
		}

		.lang-name, .lang-count, .lang-perc {
			font: 400 11px "Segoe UI", Ubuntu, Sans-Serif;
		}

		.lang-name {
			fill: {{ .Theme.LangName }};
		}

		.lang-count {
			fill: {{ .Theme.Count }};
		}

		.lang-perc {
			fill: {{ .Theme.Percent }};
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

	<rect data-testid="card-bg" x="0.5" y="0.5" rx="4.5" height="100%" stroke="{{ .Theme.CardStroke }}" width="100%"
		fill="{{ .Theme.CardBG }}" stroke-opacity="0" />

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

func process_entries[T any](tmpl *template.Template, data []T) string {
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
	data.Theme = theme
	tmpl, err := template.New("svg").Funcs(svgTmplFuncMap).Parse(SVGTEMPLATESTRING)
	check(err)

	err = tmpl.Execute(writer, data)
	check(err)
}

type CompactEntryData struct {
	LangName   string
	TotalWidth int
	XOffset    int
	YOffset    int
	CountStr   string
	PercStr    string
	Delay      int
	FillDelay  int
	RectX      int
	RectW      int
	Color      string
}

func create_compact(totalLines float64, langsSorted []LangLineByteTriplet, outputFile *os.File) {
	const MASK = `<mask id="rect-mask">
	<rect x="%d" y="0" width="%d" height="8" fill="white" rx="5" />
</mask>` + "\n"

	const SVGENTRY = `<rect mask="url(#rect-mask)" x="{{ .RectX }}" y="0" width="{{ .RectW }}" height="8" fill="{{ .Color }}" />
<g transform="translate({{ .XOffset }}, {{ .YOffset }})">
	<g class="stagger" style="animation-delay: {{ .Delay }}ms">
		<circle r="4" cx="31" cy="31" fill="{{ .Color }}" />
		<text data-testid="lang-name" x="51" y="32" class="lang-name">{{ .LangName }}</text>
		{{ if eq .CountStr "" }} {{ else }} <text x="{{ $half := div .TotalWidth 2 }}{{ sub $half 57 }}" y="32" class="lang-count">{{ .CountStr }}</text> {{ end }}
		<text x="{{ $half := div .TotalWidth 2 }}{{ sub $half 13 }}" y="32" class="lang-perc">{{ .PercStr }}%</text>
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
	entries := make([]CompactEntryData, count)

	// totalRectW := int(float64(width) * 0.8)
	// rectXInitial := int(float64(width-totalRectW) / 2)
	totalRectW := width - 50
	rectXInitial := 25
	rectX := rectXInitial

	for i, lt := range langsSorted {
		perc := float64(lt.lines) / totalLines
		percStr := fmt_double(perc * 100)
		percStrLen := len(percStr)

		if percStr[percStrLen-1] == '.' {
			percStr = percStr[:percStrLen-1]
		}

		rectW := int(math.Round(perc * float64(totalRectW)))

		// Forcibly overflow since it'll get masked off, in case it's a pixel or two short...
		if i == count-1 {
			rectW = rectW + 20
		}

		entries[i] = CompactEntryData{
			LangName:   lt.lang,
			TotalWidth: width,
			XOffset:    i % 2 * ((width / 2) - 12),
			YOffset:    i / 2 * 20,
			CountStr:   fmt_count(lt),
			PercStr:    percStr,
			Delay:      450 + i*150,
			FillDelay:  750 + i*150,
			RectX:      rectX,
			RectW:      rectW,
			Color:      enry.GetColor(lt.lang),
		}

		rectX += rectW
	}

	process_template(SVGData{
		Width:   width,
		Height:  int(math.Ceil((float64(count)/2.0)))*20 + 95,
		TitleX:  width / 2,
		Entries: fmt.Sprintf(MASK, rectXInitial, totalRectW) + process_entries(tmpl, entries),
		Styles: `.header { text-anchor: middle; }
.lang-name, .lang-perc, .lang-count { dominant-baseline: middle }
.lang-perc, .lang-count { text-anchor: end; }`,
	}, outputFile)
}

type VerticalEntryData struct {
	LangName  string
	XOffset   int
	YOffset   int
	CountStr  string
	PercStr   string
	Delay     int
	FillDelay int
	RectW     int
	Color     string
}

func create_vertical(totalLines float64, langsSorted []LangLineByteTriplet, outputFile *os.File) {
	const SVGENTRY = `<g transform="translate({{ .XOffset }}, {{ .YOffset }})">
	<g class="stagger" style="animation-delay: {{ .Delay }}ms">
		<text data-testid="lang-name" x="2" y="15" class="lang-name">{{ .LangName }} <tspan class="lang-count">({{ .CountStr }})</tspan></text>
		<text x="215" y="33" class="lang-perc">{{ .PercStr }}%</text>
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
	entries := make([]VerticalEntryData, count)

	for i, lt := range langsSorted {
		perc := float64(lt.lines) / totalLines
		percStr := fmt_double(perc * 100)
		percStrLen := len(percStr)

		if percStr[percStrLen-1] == '.' {
			percStr = percStr[:percStrLen-1]
		}

		entries[i] = VerticalEntryData{
			LangName:  lt.lang,
			XOffset:   25,
			YOffset:   i * 40,
			CountStr:  fmt_count(lt),
			PercStr:   percStr,
			Delay:     450 + i*150,
			FillDelay: 750 + i*150,
			RectW:     max(int(perc*100), 2),
			Color:     enry.GetColor(lt.lang),
		}
	}

	process_template(SVGData{
		Width:   300,
		Height:  count*40 + 85,
		TitleX:  25,
		Entries: process_entries(tmpl, entries),
	}, outputFile)
}
