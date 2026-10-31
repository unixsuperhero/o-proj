package main

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"ofam.dev/ochazuke"
)

// Tables: the project and template nouns declare catalog columns (the
// runtime noun's Columns, so a host such as nabe draws their lists as
// tables), and every row of project list and template list carries its
// cells in that order (ochazuke.ListItem.Cells and the row's cells). The
// columns are declared here once; a column change is a cells change in the
// same place.

// projectColumns are project list's columns: name never drops; origin
// drops first when narrow, then license, linked, path.
var projectColumns = []ochazuke.Column{
	{Title: "name", Min: 10},
	{Title: "origin", Priority: 4},
	{Title: "path", Priority: 1},
	{Title: "license", Priority: 3},
	{Title: "linked", Priority: 2},
}

// templateColumns are template list's columns: name, kind (dir or
// tar.gz), path.
var templateColumns = []ochazuke.Column{
	{Title: "name", Min: 10},
	{Title: "kind", Priority: 2},
	{Title: "path", Priority: 1},
}

// projectCells are a manifest row's cells: name, origin, path (with ~),
// license (licenseOf) and linked (linkedIn the default hub).
func (t *Tool) projectCells(r Row) []string {
	return []string{r.Name, r.Origin, r.Path, licenseOf(r.Dir), t.linkedIn(t.DefaultHub(), r.Name, r.Dir)}
}

// templateCells are a template's cells: name, kind, path (with ~).
func (t *Tool) templateCells(tpl Template) []string {
	return []string{tpl.Name, tpl.Kind, t.tilde(tpl.Path)}
}

// licenseOf names the license in dir: mit for an MIT LICENSE, other for
// any other LICENSE, LICENSE.md, LICENSE.txt or COPYING, none without one,
// and "" when dir is not on this machine.
func licenseOf(dir string) string {
	if _, err := os.Stat(dir); err != nil {
		return ""
	}
	for _, name := range []string{"LICENSE", "LICENSE.md", "LICENSE.txt", "COPYING"} {
		f, err := os.Open(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		defer f.Close()
		s := bufio.NewScanner(f)
		for s.Scan() {
			line := strings.TrimSpace(strings.TrimLeft(s.Text(), "# "))
			if line == "" {
				continue
			}
			if strings.HasPrefix(strings.ToLower(line), "mit license") {
				return "mit"
			}
			break
		}
		return "other"
	}
	return "none"
}

// linkedIn is whether hub/name links to dir: yes, no (nothing there), or
// other (something else is at the name; link keeps it); "" when hub is
// NoHub.
func (t *Tool) linkedIn(hub, name, dir string) string {
	if hub == NoHub {
		return ""
	}
	path := filepath.Join(t.expand(hub), name)
	fi, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return "no"
	}
	if err != nil || fi.Mode()&os.ModeSymlink == 0 {
		return "other"
	}
	target, err := os.Readlink(path)
	if err == nil && (target == dir || samePath(target, dir)) {
		return "yes"
	}
	return "other"
}
