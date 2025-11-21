# ppeb's metrics

"Simple" Go project to generate a language graph for your repositories,
intended to be run locally.

## Building

0. Have the Go compiler installed
1. `git clone https://github.com/ppebb/metrics.git`
2. `go build`

## Usage
```
./ppebtrics [OPTIONS]
 -h|--help             Display this message and exit
 -c|--config           Specify the path to your config.yml
 -o|--output           Specify the output path of your svg
 -d|--dry-run          Dry run! List the repos to be cloned and analyzed
 -s|--silent           Don't output to stdout
```

## Config

See `example.config.yml` for a template.

location (`string`): The path to store repositorites at.

indepth (`boolean`): Whether to index every commit, or just count the lines of
each file as they are in the latest commit.

counttotal (`boolean`): When true, diffs are calculated as added - removed.
When false, diffs are calculated as added + removed.

langscount (`integer`): How many languages to display.

style.theme (`string`): Path to a theme.yml file (see `./themes`).

style.type (`string`): `"compact"` or `"vertical"`.

style.count (`string`): The metric to count, `"lines"` or `"bytes"`.

style.bytesbase (`integer`): When counting bytes, whether to use metric or
binary prefixes (MB vs MiB).

style.showtotal (`boolean`): Whether to include a line displaying the total
number of lines/bytes and files beneath the header.

token (`string`): A Github access token with the repository scope, only if you
want to count private repositories.

excludeforks (`boolean`): Should forks be included in counts.

parallel (`integer`): How many goroutines to spawn at once. Higher will count
faster but may encounter network bottlenecks when cloning.

users (`[]string`): Users to count repositories of.

orgs (`[]string`): Organizations to count repositories of.

repositories (`[]string`): Repositories to count. Not subject to filters.

authors (`[]string`): When counting in-depth, the author strings used to match
commits to consider (see the `--author` option of `git-log`).

filters (`[]string`): Regex patterns used to match repositories to exclude.

commits (`[]string`): List of 6-character commit hashes to exclude.

ignore.vendor (`boolean`): Whether to ignore files identified by go-enry as vendored.

ignore.dotfiles (`boolean`): Whether to ignore files identified by go-enry as dotfiles.

ignore.binary (`boolean`): Whether to ignore files identified by go-enry as binary.

ignore.configuration (`boolean`): Whether to ignore files identified by go-enry as configuration.

ignore.image (`boolean`): Whether to ignore files identified by go-enry as images.

ignore.test (`boolean`): Whether to ignore files identified by go-enry as tests.

ignore.generated (`boolean`): Whether to ignore files identified by go-enry as generated.

ignore.langs (`[]string`): List of languages to exclude from results.

## Credit
This project is loosely based upon
[lowlighter/metrics](https://github.com/lowlighter/metrics) and
[anuraghazra/github-readme-stats](https://github.com/anuraghazra/github-readme-stats)
for SVG styling.

[go-enry](https://github.com/go-enry/go-enry) is used to identify the languages
of each file.
