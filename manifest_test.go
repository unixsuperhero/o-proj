package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ofam.dev/oden"
)

// repoAt makes a git repository with one commit at dir.
func repoAt(t *testing.T, tl *Tool, dir string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, tl, dir, "init", "--quiet", "-b", "main")
	put(t, filepath.Join(dir, "README.md"), "x\n")
	git(t, tl, dir, "add", ".")
	git(t, tl, dir, "commit", "--quiet", "-m", "init")
	return dir
}

func TestAdoptRecordsAndLinks(t *testing.T) {
	tl, out, _ := tool(t)
	dir := repoAt(t, tl, filepath.Join(tl.Home, "proj", "old"))
	upstream := filepath.Join(tl.Home, "remote.git")
	git(t, tl, dir, "remote", "add", "origin", upstream)
	if err := tl.Main([]string{"adopt", dir}); err != nil {
		t.Fatal(err, out)
	}
	entries, _ := tl.Manifest()
	if len(entries) != 1 || entries[0] != (Entry{Name: "old", URL: upstream, Path: "~/proj/old", Branch: "main", Origin: OriginAdopted}) {
		t.Fatalf("manifest %+v", entries)
	}
	if got := readlink(t, filepath.Join(tl.Home, "hub", "o", "old")); got != dir {
		t.Fatalf("link %q", got)
	}
	// again: kept, not rewritten; another directory under the same name is refused
	other := repoAt(t, tl, filepath.Join(tl.Home, "elsewhere", "old"))
	out.Reset()
	if err := tl.Main([]string{"adopt", other}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "manifest: keep old -> ~/proj/old") {
		t.Fatalf("out %q", out)
	}
	if err := tl.Main([]string{"adopt", other, "--name", "old2", "--hub", "none"}); err != nil {
		t.Fatal(err)
	}
	if entries, _ = tl.Manifest(); len(entries) != 2 || entries[1].Name != "old2" {
		t.Fatalf("manifest %+v", entries)
	}
	// not a repository of its own, not a directory
	plain := filepath.Join(tl.Home, "plain")
	os.MkdirAll(plain, 0o755)
	if err := tl.Main([]string{"adopt", plain}); err == nil {
		t.Fatal("adopted a directory with no repository")
	}
	if err := tl.Main([]string{"adopt", filepath.Join(tl.Home, "nope")}); err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("missing: %v", err)
	}
}

func TestLinkAndUnlinkFromManifest(t *testing.T) {
	tl, out, _ := tool(t)
	a := repoAt(t, tl, filepath.Join(tl.Home, "proj", "a"))
	if err := tl.SaveManifest([]Entry{{Name: "a", Path: "~/proj/a"}, {Name: "gone", Path: "~/proj/gone"}}); err != nil {
		t.Fatal(err)
	}
	hub := filepath.Join(tl.Home, "hub", "o")
	n, err := tl.LinkProjects(hub, "", true)
	if err != nil || n != (LinkCounts{Linked: 1, Missing: 1}) {
		t.Fatalf("link --all %+v %v %q", n, err, out)
	}
	if got := readlink(t, filepath.Join(hub, "a")); got != a {
		t.Fatalf("link %q", got)
	}
	if n, err = tl.LinkProjects(hub, "", true); err != nil || n.Linked != 1 {
		t.Fatalf("second link --all %+v %v", n, err)
	}
	if err := tl.Main([]string{"link", "nope"}); err == nil || !strings.Contains(err.Error(), `no repository "nope"`) {
		t.Fatalf("unknown: %v", err)
	}
	if err := tl.Main([]string{"link"}); err == nil || !strings.Contains(err.Error(), "usage") {
		t.Fatalf("usage: %v", err)
	}
	// a foreign link under a manifest name is kept by unlink
	os.Symlink(filepath.Join(tl.Home, "x"), filepath.Join(hub, "gone"))
	u, err := tl.UnlinkProjects(hub, "", true)
	if err != nil || u != (UnlinkCounts{Unlinked: 1, Kept: 1}) {
		t.Fatalf("unlink --all %+v %v", u, err)
	}
	if _, err := os.Lstat(filepath.Join(hub, "a")); !os.IsNotExist(err) {
		t.Fatal("a still linked")
	}
	if entries, _ := tl.Manifest(); len(entries) != 2 {
		t.Fatal("unlink changed the manifest")
	}
}

// bareRemote makes a bare remote with one commit and returns its path.
func bareRemote(t *testing.T, tl *Tool) string {
	t.Helper()
	work := repoAt(t, tl, filepath.Join(t.TempDir(), "seed"))
	bare := filepath.Join(t.TempDir(), "remote.git")
	git(t, tl, filepath.Dir(bare), "clone", "--quiet", "--bare", work, bare)
	return bare
}

func TestSyncClonesLinksReportsAndPulls(t *testing.T) {
	tl, out, _ := tool(t)
	bare := bareRemote(t, tl)
	if err := tl.SaveManifest([]Entry{
		{Name: "r", URL: bare, Path: "~/proj/r", Branch: "main"},
		{Name: "nourl", Path: "~/proj/nourl"},
	}); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(tl.Home, "proj", "r")
	hub := filepath.Join(tl.Home, "hub", "o")

	// a dry run changes nothing
	err := tl.Main([]string{"sync", "--dry-run"})
	if err != nil {
		t.Fatal(err, out)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("dry run cloned")
	}
	if _, err := os.Stat(hub); !os.IsNotExist(err) {
		t.Fatal("dry run linked")
	}
	if !strings.Contains(out.String(), "r: would clone into ~/proj/r") || !strings.Contains(out.String(), "nourl: missing ~/proj/nourl (no url to clone from)") {
		t.Fatalf("dry run %q", out)
	}

	out.Reset()
	if err := tl.Main([]string{"clone"}); err != nil {
		t.Fatal(err, out)
	}
	if got := readlink(t, filepath.Join(hub, "r")); got != dir {
		t.Fatalf("link %q", got)
	}
	if !strings.Contains(out.String(), "r: cloned into ~/proj/r clean") {
		t.Fatalf("clone %q", out)
	}

	// upstream moves on; a dirty clone is reported and never pulled
	other := filepath.Join(t.TempDir(), "other")
	git(t, tl, filepath.Dir(other), "clone", "--quiet", bare, other)
	put(t, filepath.Join(other, "more.txt"), "more")
	git(t, tl, other, "add", ".")
	git(t, tl, other, "commit", "--quiet", "-m", "more")
	git(t, tl, other, "push", "--quiet")
	put(t, filepath.Join(dir, "scratch.txt"), "dirty")
	out.Reset()
	if err := tl.Main([]string{"sync", "--pull", "r"}); err != nil {
		t.Fatal(err, out)
	}
	if !strings.Contains(out.String(), "r: dirty (1 changed) behind 1\n") {
		t.Fatalf("dirty sync %q", out)
	}
	os.Remove(filepath.Join(dir, "scratch.txt"))
	out.Reset()
	results, err := tl.SyncEntries(context.Background(), []string{"r"}, SyncOptions{Hub: hub, Fetch: true, Pull: true})
	if err != nil || len(results) != 1 || !results[0].Pulled || results[0].Behind != 0 {
		t.Fatalf("pull %+v %v %q", results, err, out)
	}
	if _, err := os.Stat(filepath.Join(dir, "more.txt")); err != nil {
		t.Fatal("not fast-forwarded")
	}
	// an unknown name is an error; a failed clone exits 1 after the report
	if err := tl.Main([]string{"sync", "zzz"}); err == nil {
		t.Fatal("unknown name")
	}
	tl.SaveManifest([]Entry{{Name: "bad", URL: filepath.Join(tl.Home, "no-such.git"), Path: "~/proj/bad"}})
	out.Reset()
	if err := tl.Main([]string{"sync"}); oden.ExitCode(err) != 1 || !strings.Contains(out.String(), "bad: missing ~/proj/bad (clone failed)") {
		t.Fatalf("failed clone %v %q", err, out)
	}
}

func TestSeedPrecedenceConflictsAndMtmExport(t *testing.T) {
	tl, out, _ := tool(t)
	a := repoAt(t, tl, filepath.Join(tl.Home, "proj", "a"))
	b := repoAt(t, tl, filepath.Join(tl.Home, "proj", "b"))
	c := repoAt(t, tl, filepath.Join(tl.Home, "proj", "c"))
	hub := filepath.Join(tl.Home, "hub", "o")
	os.MkdirAll(hub, 0o755)
	os.Symlink(a, filepath.Join(hub, "a"))
	inv := filepath.Join(tl.Home, "repos.json")
	put(t, inv, `[{"name":"a","path":"~/proj/a"},{"name":"b","path":"~/proj/b","roots":["x"]}]`)
	export := filepath.Join(tl.Home, "mtm.json")
	put(t, export, `[{"name":"c","origin":"o-proj","default_tree":"`+c+`"},{"name":"work","origin":null,"default_tree":"/elsewhere"}]`)

	if err := tl.Main([]string{"seed", "--inventory", inv, "--mtm-json", export, "--dry-run"}); err != nil {
		t.Fatal(err, out)
	}
	if entries, _ := tl.Manifest(); len(entries) != 0 {
		t.Fatal("dry run wrote")
	}
	if !strings.Contains(out.String(), "seed: a -> ~/proj/a (hub, o-inventory)") {
		t.Fatalf("dry run %q", out)
	}
	out.Reset()
	if err := tl.Main([]string{"seed", "--inventory", inv, "--mtm-json", export}); err != nil {
		t.Fatal(err, out)
	}
	entries, _ := tl.Manifest()
	got := map[string]Entry{}
	for _, e := range entries {
		got[e.Name] = e
	}
	if len(entries) != 3 || got["a"].Origin != OriginAdopted || got["b"].Path != "~/proj/b" || got["c"].Origin != OriginNew || got["c"].Branch != "main" {
		t.Fatalf("seeded %+v", entries)
	}
	// seeding again adds nothing; a conflicting source writes nothing and exits 1
	out.Reset()
	if err := tl.Main([]string{"seed", "--inventory", inv}); err != nil || !strings.Contains(out.String(), "seed: nothing to add") {
		t.Fatalf("again %v %q", err, out)
	}
	put(t, inv, `[{"name":"b","path":"~/proj/a"},{"name":"d","path":"~/proj/d"}]`)
	out.Reset()
	err := tl.Main([]string{"seed", "--inventory", inv})
	if oden.ExitCode(err) != 1 || !strings.Contains(out.String(), "conflict: b: manifest says ~/proj/b, o-inventory says ~/proj/a") {
		t.Fatalf("conflict %v %q", err, out)
	}
	if entries, _ := tl.Manifest(); len(entries) != 3 {
		t.Fatal("a conflicting seed wrote")
	}
	_ = b
}

// fakeMtm writes an mtm script answering catalog (with verbs), projects
// directories (from a state file) and project new (appending to it).
func fakeMtm(t *testing.T, tl *Tool, verbs string, fail bool) (calls, state string) {
	t.Helper()
	bin := t.TempDir()
	state = filepath.Join(t.TempDir(), "dirs")
	calls = filepath.Join(t.TempDir(), "calls")
	put(t, state, "")
	newCmd := `printf '%s\n' "$5" >> ` + state + `; echo '{"id":"p1","name":"'$3'"}'`
	if fail {
		newCmd = `echo boom >&2; exit 3`
	}
	script := `#!/bin/sh
echo "$*" >> ` + calls + `
case "$1 $2" in
"catalog "*) echo '{"nouns":[` + verbs + `]}' ;;
"projects directories")
  printf '['; sep=''
  while read -r d; do printf '%s{"id":"x","label":"old","data":{"directories":["%s"]}}' "$sep" "$d"; sep=','; done < ` + state + `
  echo ']' ;;
"project new") ` + newCmd + ` ;;
esac
`
	put(t, filepath.Join(bin, "mtm"), script)
	os.Chmod(filepath.Join(bin, "mtm"), 0o755)
	setEnv(tl, "PATH", bin+string(filepath.ListSeparator)+testPath(t))
	return calls, state
}

const fullVerbs = `{"name":"project","verbs":[{"name":"new"}]},{"name":"projects","verbs":[{"name":"directories"}]}`

func TestNewCreatesMtmProjectOnce(t *testing.T) {
	tl, out, _ := tool(t)
	calls, _ := fakeMtm(t, tl, fullVerbs, false)
	root := filepath.Join(tl.Home, "proj")
	if err := tl.Main([]string{"new", "demo", "--root", root}); err != nil {
		t.Fatal(err, out)
	}
	dir := filepath.Join(root, "demo")
	if !strings.Contains(out.String(), "mtm: project demo (p1) for "+dir) {
		t.Fatalf("out %q", out)
	}
	out.Reset()
	if err := tl.Main([]string{"new", "demo", "--root", root, "--existing"}); err != nil {
		t.Fatal(err, out)
	}
	if !strings.Contains(out.String(), "mtm: project old (x) already has "+dir) {
		t.Fatalf("second %q", out)
	}
	log, _ := os.ReadFile(calls)
	if n := strings.Count(string(log), "project new demo --dir "+dir); n != 1 {
		t.Fatalf("project new ran %d times: %s", n, log)
	}
}

func TestNewMtmSeesHubLinkSpelling(t *testing.T) {
	tl, out, _ := tool(t)
	_, state := fakeMtm(t, tl, fullVerbs, false)
	root := filepath.Join(tl.Home, "proj")
	dir := filepath.Join(root, "demo")
	os.MkdirAll(filepath.Join(tl.Home, "hub", "o"), 0o755)
	os.MkdirAll(dir, 0o755)
	os.Symlink(dir, filepath.Join(tl.Home, "hub", "o", "demo"))
	// mtm already has the project under the hub-link spelling
	put(t, state, filepath.Join(tl.Home, "hub", "o", "demo")+"\n")
	if err := tl.Main([]string{"new", "demo", "--root", root, "--existing"}); err != nil {
		t.Fatal(err, out)
	}
	if !strings.Contains(out.String(), "mtm: project old (x) already has "+dir) {
		t.Fatalf("out %q", out)
	}
}

func TestNewMtmQuietOrWarns(t *testing.T) {
	for _, c := range []struct {
		name, verbs string
		fail        bool
		want        string
	}{
		{"no project.new", `{"name":"project","verbs":[{"name":"open"}]}`, false, "hook: skipped (o-inventory not on PATH)\ncd "},
		{"no directories verb", `{"name":"project","verbs":[{"name":"new"}]}`, false, "mtm: skipped (mtm has no projects directories verb"},
		{"new fails", fullVerbs, true, "mtm: warning: project new failed"},
	} {
		t.Run(c.name, func(t *testing.T) {
			tl, out, _ := tool(t)
			fakeMtm(t, tl, c.verbs, c.fail)
			if err := tl.Main([]string{"new", "demo", "--root", filepath.Join(tl.Home, "proj")}); err != nil {
				t.Fatal(err, out)
			}
			if !strings.Contains(out.String(), c.want) || !strings.HasPrefix(lastLine(out.String()), "cd ") {
				t.Fatalf("out %q", out)
			}
		})
	}
	tl, out, _ := tool(t)
	calls, _ := fakeMtm(t, tl, fullVerbs, false)
	if err := tl.Main([]string{"new", "demo", "--root", filepath.Join(tl.Home, "proj"), "--no-mtm"}); err != nil {
		t.Fatal(err, out)
	}
	if _, err := os.Stat(calls); !os.IsNotExist(err) {
		t.Fatal("--no-mtm ran mtm")
	}
}

func TestProjectListRowsAreManifest(t *testing.T) {
	tl, out, _ := tool(t)
	tl.SaveManifest([]Entry{{Name: "b", Path: "~/b"}, {Name: "a", Path: "/abs/a", URL: "u"}})
	if err := tl.Main([]string{"list"}); err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if err := json.Unmarshal(out.Bytes(), &rows); err != nil || len(rows) != 2 || rows[0].Name != "a" || rows[1].Dir != filepath.Join(tl.Home, "b") {
		t.Fatalf("rows %+v %v", rows, err)
	}
}
