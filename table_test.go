package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// TestCatalogDeclaresTableColumns pins the columns project and template
// publish in the catalog.
func TestCatalogDeclaresTableColumns(t *testing.T) {
	tl, out, _ := tool(t)
	if err := tl.Main([]string{"catalog"}); err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Nouns []struct {
			Name    string `json:"name"`
			Columns []struct {
				Title string `json:"title"`
			} `json:"columns"`
			Verbs []struct {
				Name string `json:"name"`
				Rows string `json:"rows"`
			} `json:"verbs"`
		} `json:"nouns"`
	}
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatal(err, out)
	}
	got := map[string][]string{}
	rows := map[string]string{}
	for _, n := range doc.Nouns {
		for _, c := range n.Columns {
			got[n.Name] = append(got[n.Name], c.Title)
		}
		for _, v := range n.Verbs {
			if v.Name == "list" {
				rows[n.Name] = v.Rows
			}
		}
	}
	if want := []string{"name", "origin", "path", "license", "linked"}; !slices.Equal(got["project"], want) {
		t.Fatalf("project columns %v, want %v", got["project"], want)
	}
	if want := []string{"name", "kind", "path"}; !slices.Equal(got["template"], want) {
		t.Fatalf("template columns %v, want %v", got["template"], want)
	}
	if rows["template"] != "template" {
		t.Fatalf("template list rows %q", rows["template"])
	}
}

// TestProjectListCells: one cell per column, license and linked read from
// the disk and the hub.
func TestProjectListCells(t *testing.T) {
	tl, out, _ := tool(t)
	hub := filepath.Join(tl.Home, "hub", "o")
	setEnv(tl, "O_PROJ_HUB", hub)
	mk := func(name, license string) string {
		dir := filepath.Join(tl.Home, "proj", name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if license != "" {
			if err := os.WriteFile(filepath.Join(dir, "LICENSE"), []byte(license), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return dir
	}
	a := mk("a", "\nMIT License\n\nCopyright\n")
	mk("b", "Apache License\n")
	mk("c", "")
	if err := os.MkdirAll(filepath.Join(hub, "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(a, filepath.Join(hub, "a")); err != nil {
		t.Fatal(err)
	}
	tl.SaveManifest([]Entry{
		{Name: "a", Path: "~/proj/a", Origin: OriginNew},
		{Name: "b", Path: "~/proj/b", Origin: OriginAdopted},
		{Name: "c", Path: "~/proj/c"},
		{Name: "gone", Path: "~/proj/gone", Origin: OriginAdopted},
	})
	if err := tl.Main([]string{"project", "list", "--json", "{}"}); err != nil {
		t.Fatal(err, out)
	}
	var rows []projectRow
	if err := json.Unmarshal(out.Bytes(), &rows); err != nil {
		t.Fatal(err, out)
	}
	want := map[string][]string{
		"a":    {"a", "o-proj", "~/proj/a", "mit", "yes"},
		"b":    {"b", "adopted", "~/proj/b", "other", "other"},
		"c":    {"c", "", "~/proj/c", "none", "no"},
		"gone": {"gone", "adopted", "~/proj/gone", "", "no"},
	}
	if len(rows) != len(want) {
		t.Fatalf("rows %+v", rows)
	}
	for _, r := range rows {
		if !slices.Equal(r.Cells, want[r.ID]) || len(r.Cells) != len(projectColumns) {
			t.Fatalf("%s cells %q, want %q", r.ID, r.Cells, want[r.ID])
		}
	}
}

// TestTemplateListRows: template list answers the catalog as rows with
// cells, and an empty catalog as an empty list.
func TestTemplateListRows(t *testing.T) {
	tl, out, _ := tool(t)
	if err := tl.Main([]string{"template", "list", "--json", "{}"}); err != nil {
		t.Fatal(err, out)
	}
	var rows []templateRow
	if err := json.Unmarshal(out.Bytes(), &rows); err != nil || len(rows) != 0 {
		t.Fatalf("empty %q %v", out, err)
	}
	catalog(t, tl)
	out.Reset()
	if err := tl.Main([]string{"template", "list", "--json", "{}"}); err != nil {
		t.Fatal(err, out)
	}
	if err := json.Unmarshal(out.Bytes(), &rows); err != nil {
		t.Fatal(err, out)
	}
	dir := tl.tilde(tl.TemplateDir())
	want := [][]string{{"packed", "tar.gz", dir + "/packed.tar.gz"}, {"plain", "dir", dir + "/plain"}}
	if len(rows) != 2 {
		t.Fatalf("rows %+v", rows)
	}
	for i, r := range rows {
		if r.ID != want[i][0] || r.Data.Name != want[i][0] || !slices.Equal(r.Cells, want[i]) {
			t.Fatalf("row %d %+v, want cells %q", i, r, want[i])
		}
	}
}
