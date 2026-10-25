package main

import (
	"bytes"
	"encoding/json"
	"strings"
)

// o proj new asks mtm for a work project for the new repository, through
// mtm's command line only (never its database): mtm's catalog must offer
// project.new, mtm's projects directories rows must show no project that
// already has the directory, then mtm project new NAME --dir DIR. Every
// failure is one line of output, never an error: the repository already
// exists (record first). If mtm drops the noun or the verb, this quietly
// stops. $O_PROJ_MTM names the mtm binary (default: mtm on PATH) and
// $O_PROJ_MTM_ARGS words to put before every mtm command (--data PATH).

// mtmCommand is the mtm binary and its leading words; bin is "" when there
// is no mtm.
func (t *Tool) mtmCommand() (string, []string) {
	bin := lookup(t.Env, "O_PROJ_MTM")
	if bin == "" {
		bin = t.lookPath("mtm")
	}
	return bin, strings.Fields(lookup(t.Env, "O_PROJ_MTM_ARGS"))
}

// mtmCatalog is the part of an ochazuke catalog this reads.
type mtmCatalog struct {
	Nouns []struct {
		Name  string `json:"name"`
		Verbs []struct {
			Name string `json:"name"`
		} `json:"verbs"`
	} `json:"nouns"`
}

func (c mtmCatalog) has(noun, verb string) bool {
	for _, n := range c.Nouns {
		if n.Name != noun {
			continue
		}
		for _, v := range n.Verbs {
			if v.Name == verb {
				return true
			}
		}
	}
	return false
}

// mtmProjectRow is one row of mtm projects directories.
type mtmProjectRow struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Data  struct {
		Directories []string `json:"directories"`
	} `json:"data"`
}

// mtmProject creates mtm's work project for dir unless one already has it.
func (t *Tool) mtmProject(name, dir string) {
	bin, lead := t.mtmCommand()
	if bin == "" {
		t.say("mtm: skipped (mtm not on PATH)")
		return
	}
	mtm := func(args ...string) ([]byte, error) {
		return t.Run(t.Home, bin, append(append([]string(nil), lead...), args...)...)
	}
	out, err := mtm("catalog")
	var cat mtmCatalog
	if err == nil {
		err = json.Unmarshal(out, &cat)
	}
	if err != nil {
		t.say("mtm: skipped (mtm catalog: %v)", err)
		return
	}
	if !cat.has("project", "new") {
		return // mtm has no project.new: nothing to ask for
	}
	if !cat.has("projects", "directories") {
		t.say("mtm: skipped (mtm has no projects directories verb to check for an existing project)")
		return
	}
	out, err = mtm("projects", "directories")
	if err != nil {
		t.say("mtm: skipped (mtm projects directories: %v)", err)
		return
	}
	rows, err := parseRows(out)
	if err != nil {
		t.say("mtm: skipped (mtm projects directories: %v)", err)
		return
	}
	key := dirKey(dir)
	for _, r := range rows {
		for _, d := range r.Data.Directories {
			if dirKey(d) == key {
				t.say("mtm: project %s (%s) already has %s", r.Label, r.ID, dir)
				return
			}
		}
	}
	out, err = mtm("project", "new", name, "--dir", dir)
	if err != nil {
		t.say("mtm: warning: project new failed (%v)", err)
		return
	}
	var p struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(out, &p)
	if p.ID == "" {
		t.say("mtm: project %s for %s", name, dir)
		return
	}
	t.say("mtm: project %s (%s) for %s", name, p.ID, dir)
}

// parseRows reads a JSON array of rows or JSON Lines of rows.
func parseRows(out []byte) ([]mtmProjectRow, error) {
	out = bytes.TrimSpace(out)
	var rows []mtmProjectRow
	if bytes.HasPrefix(out, []byte("[")) {
		err := json.Unmarshal(out, &rows)
		return rows, err
	}
	for _, line := range bytes.Split(out, []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var r mtmProjectRow
		if err := json.Unmarshal(line, &r); err != nil {
			return nil, err
		}
		rows = append(rows, r)
	}
	return rows, nil
}

// dirKey is a directory's identity: absolute, cleaned, symlinks resolved
// when it resolves (mtm's directoryKey), so a hub-link spelling and the
// real path are one directory.
func dirKey(p string) string { return real(p) }
