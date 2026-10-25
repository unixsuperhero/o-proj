package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"
)

// List prints the manifest: JSON rows (name, url, path, branch, origin and
// the expanded dir), or a table with --table.
func (t *Tool) List(args []string) error {
	flags := flag.NewFlagSet(t.Prog+" list", flag.ContinueOnError)
	flags.SetOutput(t.Stderr)
	table := flags.Bool("table", false, "a table for people instead of JSON")
	flags.BoolVar(table, "t", false, "alias for --table")
	words, err := parse(flags, args)
	if err != nil {
		return err
	}
	if len(words) > 0 {
		return fmt.Errorf("usage: %s list [--table]", t.Prog)
	}
	entries, err := t.Manifest()
	if err != nil {
		return err
	}
	rows := t.rows(entries)
	if !*table {
		enc := json.NewEncoder(t.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(rows)
	}
	w := tabwriter.NewWriter(t.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tPATH\tBRANCH\tORIGIN\tURL")
	for _, r := range rows {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", r.Name, r.Path, r.Branch, r.Origin, r.URL)
	}
	return w.Flush()
}

// AdoptResult is what adopt recorded: the entry and the manifest outcome
// (recorded, exists or taken).
type AdoptResult struct {
	Entry   Entry  `json:"entry"`
	Outcome string `json:"outcome"`
}

// Adopt records an existing repository in the manifest and links it from
// the hub. It touches no file in the repository. PATH must be the toplevel
// of a git repository of its own (the guard new uses).
func (t *Tool) Adopt(args []string) error {
	flags := flag.NewFlagSet(t.Prog+" adopt", flag.ContinueOnError)
	flags.SetOutput(t.Stderr)
	name := flags.String("name", "", "the manifest name (default: the directory's base name)")
	hub := flags.String("hub", t.DefaultHub(), "the hub directory that links NAME -> DIR; none skips the link")
	words, err := parse(flags, args)
	if err != nil {
		return err
	}
	if len(words) != 1 {
		return fmt.Errorf("usage: %s adopt PATH [--name NAME] [--hub DIR|none]", t.Prog)
	}
	_, err = t.AdoptPath(words[0], *name, *hub)
	return err
}

// AdoptPath is adopt's work, shared with the project noun.
func (t *Tool) AdoptPath(path, name, hub string) (AdoptResult, error) {
	dir, err := filepath.Abs(t.expand(path))
	if err != nil {
		return AdoptResult{}, err
	}
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return AdoptResult{}, fmt.Errorf("%s is not a directory; adopt takes the path of an existing repository (to make one: %s new NAME)", dir, t.Prog)
	}
	if _, err := t.verify(dir); err != nil {
		return AdoptResult{}, fmt.Errorf("%s is not the toplevel of its own git repository: %w", dir, err)
	}
	if name == "" {
		name = filepath.Base(dir)
	}
	e := t.describe(name, dir, OriginAdopted)
	outcome, err := t.record(e)
	if err != nil {
		return AdoptResult{}, err
	}
	if hub != NoHub && outcome != "taken" {
		if _, err := t.link(hub, name, dir); err != nil {
			return AdoptResult{}, err
		}
	}
	return AdoptResult{Entry: e, Outcome: outcome}, nil
}
