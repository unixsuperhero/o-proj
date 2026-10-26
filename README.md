# o-proj

The family's repositories on disk: `o proj` (or `o-proj`). Moved out of matomeru (`cmd/o-proj`, history kept by `git subtree split`); folds in o-family's status report.

```sh
o proj new NAME [--root DIR] [--template T] [--existing] [--github] [--public] [--license mit|none] [--hub DIR|none] [--no-mtm]
o proj adopt PATH [--name NAME]         # record an existing repository and link it
o proj list [--table]                   # the manifest (JSON by default)
o proj link NAME | --all                # ~/hub/o/NAME -> DIR, from the manifest
o proj unlink NAME | --all
o proj clone [NAME...]                  # clone what is missing on this machine, then link
o proj sync [NAME...] [--pull] [--dry-run]   # clone, link, fetch, report dirty/behind; --pull fast-forwards clean ones
o proj status [DIR...] [-d|--dirty] [-c|--clean] [-j|--json]
o proj seed [--mtm-json FILE] [--inventory FILE] [--hub DIR] [--dry-run]
o proj check [DIR]                      # refuse a directory whose git toplevel is $HOME or above ~/proj
o proj templates
o proj catalog                          # the project and template nouns, for nabe
```

`o proj project list` and `o proj template list` answer rows (`{id, label, data, cells}`) that a host draws as tables: the catalog declares each noun's `columns` and every row carries one cell per column.

| Noun | Columns (name never drops; when narrow, project drops origin, license, linked, then path; template drops kind, then path) |
| --- | --- |
| project | name, origin, path (with `~`), license (`mit`, `other`, `none`; empty when the directory is not on this machine), linked (`yes`, `no`, or `other` when something else is at `~/hub/o/NAME`) |
| template | name, kind (`dir` or `tar.gz`), path |

## The manifest

`~/hub/repos.json` (`$O_PROJ_MANIFEST`), a JSON array sorted by name:

```json
[{ "name": "oden", "url": "git@github.com:unixsuperhero/oden.git", "path": "~/proj/oden", "branch": "main", "origin": "adopted" }]
```

`path` keeps `~` and is expanded per machine; `origin` is `o-proj` (made by `new`) or `adopted`. `list` adds the expanded `dir`. The hub links under `~/hub/o` are generated from it (`link --all`, `sync`) and are not committed.

## mtm

`o proj new` asks mtm for a work project, through its command line only: when `mtm catalog` offers `project.new` and `mtm projects directories` shows no project with this directory, it runs `mtm project new NAME --dir DIR`. `--no-mtm` skips it; any failure is a warning. `$O_PROJ_MTM` and `$O_PROJ_MTM_ARGS` (for example `--data /tmp/x.db`) point it at another mtm.

## Build

```sh
gofmt -l . && go vet ./... && go test -race ./...
go build -o ~/bin/o-proj .
```

MIT license.
