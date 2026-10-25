package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readlink(t *testing.T, path string) string {
	t.Helper()
	target, err := os.Readlink(path)
	if err != nil {
		t.Fatal(err)
	}
	return target
}

func TestNewLinksFromHub(t *testing.T) {
	tl, out, _ := tool(t)
	root := filepath.Join(tl.Home, "proj")
	hub := filepath.Join(t.TempDir(), "hub", "o") // created by new
	if err := tl.Main([]string{"new", "demo", "--root", root, "--hub", hub, "--no-register"}); err != nil {
		t.Fatal(err, out)
	}
	dir := filepath.Join(root, "demo")
	if got := readlink(t, filepath.Join(hub, "demo")); got != dir {
		t.Fatalf("link -> %q, want %q", got, dir)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if got := lines[len(lines)-4]; got != "link: "+filepath.Join(hub, "demo")+" -> "+dir {
		t.Fatalf("link line %q", got)
	}
	if !strings.HasPrefix(lines[len(lines)-3], "hook: ") || lines[len(lines)-2] != "mtm: skipped (--no-mtm)" {
		t.Fatalf("hook and mtm after link: %q", lines)
	}
	if !strings.HasPrefix(lines[len(lines)-5], "manifest: demo -> ") {
		t.Fatalf("manifest before link: %q", lines)
	}
}

func TestNewDefaultHubIsHomeHubO(t *testing.T) {
	tl, out, _ := tool(t)
	if err := tl.Main([]string{"new", "x", "--root", filepath.Join(tl.Home, "proj"), "--no-register"}); err != nil {
		t.Fatal(err, out)
	}
	if got := readlink(t, filepath.Join(tl.Home, "hub", "o", "x")); got != filepath.Join(tl.Home, "proj", "x") {
		t.Fatalf("link -> %q", got)
	}
	if !strings.Contains(out.String(), "link: ~/hub/o/x -> "+filepath.Join(tl.Home, "proj", "x")) {
		t.Fatalf("out %q", out)
	}
	tl.Env = append(tl.Env, "O_PROJ_HUB="+filepath.Join(tl.Home, "elsewhere"))
	if err := tl.Main([]string{"new", "y", "--root", filepath.Join(tl.Home, "proj"), "--no-register"}); err != nil {
		t.Fatal(err, out)
	}
	if _, err := os.Lstat(filepath.Join(tl.Home, "elsewhere", "y")); err != nil {
		t.Fatal(err)
	}
}

func TestNewFamilyFlags(t *testing.T) {
	for _, flag := range []string{"--family", "-o", "--family=false", "-o=false"} {
		t.Run(flag, func(t *testing.T) {
			tl, out, _ := tool(t)
			root := filepath.Join(tl.Home, "proj")
			args := []string{"new", "demo", "--root", root, "--no-register", flag}
			if err := tl.Main(args); err != nil {
				t.Fatal(err, out)
			}
			path := filepath.Join(tl.Home, "hub", "o", "demo")
			if strings.HasSuffix(flag, "=false") {
				if _, err := os.Lstat(path); !os.IsNotExist(err) {
					t.Fatalf("family link should be absent: %v", err)
				}
				return
			}
			want := filepath.Join(root, "demo")
			if got := readlink(t, path); got != want {
				t.Fatalf("link -> %q, want %q", got, want)
			}
			if err := tl.Main(append(args, "--existing")); err != nil {
				t.Fatal(err, out)
			}
			if got := readlink(t, path); got != want {
				t.Fatalf("repeat link -> %q, want %q", got, want)
			}
		})
	}
}

func TestNewHubNoneSkips(t *testing.T) {
	tl, out, _ := tool(t)
	if err := tl.Main([]string{"new", "x", "--root", filepath.Join(tl.Home, "proj"), "--hub", "none", "--no-register"}); err != nil {
		t.Fatal(err, out)
	}
	if _, err := os.Lstat(filepath.Join(tl.Home, "hub", "o")); !os.IsNotExist(err) {
		t.Fatalf("hub made: %v", err)
	}
	if !strings.Contains(out.String(), "link: skipped (--hub none)") {
		t.Fatalf("out %q", out)
	}
}

func TestNewKeepsForeignLink(t *testing.T) {
	tl, out, _ := tool(t)
	hub := filepath.Join(tl.Home, "hub", "o")
	if err := os.MkdirAll(hub, 0o755); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(tl.Home, "other")
	if err := os.Symlink(other, filepath.Join(hub, "demo")); err != nil {
		t.Fatal(err)
	}
	if err := tl.Main([]string{"new", "demo", "--root", filepath.Join(tl.Home, "proj"), "--no-register"}); err != nil {
		t.Fatal(err, out)
	}
	if got := readlink(t, filepath.Join(hub, "demo")); got != other {
		t.Fatalf("overwrote: -> %q", got)
	}
	if !strings.Contains(out.String(), "keep: ~/hub/o/demo exists -> "+other) {
		t.Fatalf("out %q", out)
	}
	if got := lastLine(out.String()); !strings.HasPrefix(got, "cd ") {
		t.Fatalf("did not continue: %q", got)
	}
}

func TestNewAgentsPointsAtPlans(t *testing.T) {
	tl, out, _ := tool(t)
	root := filepath.Join(tl.Home, "proj")
	if err := tl.Main([]string{"new", "a", "--root", root, "--no-register"}); err != nil {
		t.Fatal(err, out)
	}
	want := "# a\n\nCreated by o-proj.\n\n## Plans and todos\n\n" +
		"Family plans, todos and the roadmap live in `~/hub/o-plans/` (`README.md` is the index). " +
		"Read it before planning work; add or update items there. " +
		"Do not add a TODO.md or docs/plans/ to this repo.\n"
	data, err := os.ReadFile(filepath.Join(root, "a", "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != want {
		t.Fatalf("AGENTS.md:\n%s\nwant:\n%s", data, want)
	}

	outside := t.TempDir()
	for _, c := range []struct{ hub, plans string }{
		{filepath.Join(tl.Home, "work", "links"), "~/work/o-plans"},
		{"~/notes/o", "~/notes/o-plans"},
		{filepath.Join(outside, "o"), filepath.Join(outside, "o-plans")},
	} {
		tl.Env = append(tl.Env, "O_PROJ_HUB="+c.hub)
		if got := tl.PlansDir(); got != c.plans {
			t.Errorf("O_PROJ_HUB=%s: plans %q, want %q", c.hub, got, c.plans)
		}
	}
}
