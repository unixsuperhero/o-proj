package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/unixsuperhero/oden"
)

// Seed imports repositories into the manifest once, from what recorded
// them before the manifest existed: the hub's links, o-inventory's
// repos.json, and a saved o-proj list from the time o-proj wrote mtm's
// store (--mtm-json; only rows with origin o-proj). It adds missing names
// only and never rewrites an entry. A name's directory comes from the first
// source that has it, in this order: the manifest, the hub link, repos.json,
// the mtm export. Sources that disagree on a name's directory are printed
// as conflict lines and nothing is written (exit 1); adopt the right
// directory first, then seed again.
func (t *Tool) Seed(args []string) error {
	flags := flag.NewFlagSet(t.Prog+" seed", flag.ContinueOnError)
	flags.SetOutput(t.Stderr)
	hub := flags.String("hub", t.DefaultHub(), "the hub directory whose links are read; none skips")
	config := lookup(t.Env, "XDG_CONFIG_HOME")
	if config == "" {
		config = filepath.Join(t.Home, ".config")
	}
	inventory := flags.String("inventory", filepath.Join(config, "o-inventory", "repos.json"), "o-inventory's repos.json; none skips")
	mtmJSON := flags.String("mtm-json", "", "a saved o-proj list (the version that read mtm's store); rows with origin o-proj are read")
	dryRun := flags.Bool("dry-run", false, "print what would be added; write nothing")
	flags.BoolVar(dryRun, "n", false, "alias for --dry-run")
	words, err := parse(flags, args)
	if err != nil {
		return err
	}
	if len(words) > 0 {
		return fmt.Errorf("usage: %s seed [--hub DIR|none] [--inventory FILE|none] [--mtm-json FILE] [--dry-run]", t.Prog)
	}
	type candidate struct{ source, dir string }
	found := map[string][]candidate{}
	madeByOProj := map[string]bool{}
	add := func(source, name, dir string) {
		if name != "" && dir != "" {
			found[name] = append(found[name], candidate{source, dir})
		}
	}
	entries, err := t.Manifest()
	if err != nil {
		return err
	}
	for _, e := range entries {
		add("manifest", e.Name, t.Dir(e))
	}
	if *hub != NoHub {
		dir := t.expand(*hub)
		links, err := os.ReadDir(dir)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		for _, l := range links {
			if l.Type()&os.ModeSymlink == 0 {
				continue
			}
			target, err := os.Readlink(filepath.Join(dir, l.Name()))
			if err != nil {
				return err
			}
			if !filepath.IsAbs(target) {
				target = filepath.Join(dir, target)
			}
			add("hub", l.Name(), target)
		}
	}
	if *inventory != NoHub {
		data, err := os.ReadFile(t.expand(*inventory))
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if err == nil {
			var repos []struct{ Name, Path string }
			if err := json.Unmarshal(data, &repos); err != nil {
				return fmt.Errorf("%s: %w", *inventory, err)
			}
			for _, r := range repos {
				add("o-inventory", r.Name, t.expand(r.Path))
			}
		}
	}
	if *mtmJSON != "" {
		data, err := os.ReadFile(t.expand(*mtmJSON))
		if err != nil {
			return err
		}
		var projects []struct {
			Name        string `json:"name"`
			Origin      string `json:"origin"`
			DefaultTree string `json:"default_tree"`
			Trees       []struct {
				Path string `json:"path"`
			} `json:"trees"`
		}
		if err := json.Unmarshal(data, &projects); err != nil {
			return fmt.Errorf("%s: %w", *mtmJSON, err)
		}
		for _, p := range projects {
			if p.Origin != OriginNew {
				continue
			}
			madeByOProj[p.Name] = true
			dir := p.DefaultTree
			if dir == "" && len(p.Trees) > 0 {
				dir = p.Trees[0].Path
			}
			add("mtm", p.Name, dir)
		}
	}

	names := make([]string, 0, len(found))
	for n := range found {
		names = append(names, n)
	}
	sort.Strings(names)
	var added []Entry
	conflicts := 0
	for _, name := range names {
		cands := found[name]
		chosen := cands[0]
		var sources []string
		for _, c := range cands {
			sources = append(sources, c.source)
			if dirKey(c.dir) != dirKey(chosen.dir) {
				t.say("conflict: %s: %s says %s, %s says %s", name, chosen.source, t.tilde(chosen.dir), c.source, t.tilde(c.dir))
				conflicts++
			}
		}
		if chosen.source == "manifest" {
			continue
		}
		origin := OriginAdopted
		if madeByOProj[name] {
			origin = OriginNew
		}
		dir := dirKey(chosen.dir)
		e := Entry{Name: name, Path: t.tilde(dir), Origin: origin}
		if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
			e = t.describe(name, dir, origin)
		}
		added = append(added, e)
		t.say("seed: %s -> %s (%s)", name, e.Path, strings.Join(sources, ", "))
	}
	if conflicts > 0 {
		t.say("seed: %d conflicts; nothing written. Adopt the right directory for each (%s adopt PATH --name NAME), then seed again", conflicts, t.Prog)
		return &oden.ExitError{Code: 1}
	}
	if *dryRun {
		t.say("seed: dry run: %d to add to %s", len(added), t.tilde(t.ManifestPath()))
		return nil
	}
	if len(added) == 0 {
		t.say("seed: nothing to add")
		return nil
	}
	if err := t.SaveManifest(append(entries, added...)); err != nil {
		return err
	}
	t.say("seed: added %d to %s", len(added), t.tilde(t.ManifestPath()))
	return nil
}
