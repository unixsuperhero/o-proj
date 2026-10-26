package main

import (
	"encoding/json"
	"testing"

	"github.com/unixsuperhero/ochazuke"
	ocli "github.com/unixsuperhero/ochazuke/cli"
)

// Every verb o-proj defines declares its history kind (Operation.HistoryKind)
// rather than leaving it to ochazuke's default, and the catalog's history is
// that kind. A new verb without one fails here.
func TestEveryVerbDeclaresItsHistory(t *testing.T) {
	tl, out, _ := tool(t)
	if err := tl.Main([]string{"catalog"}); err != nil {
		t.Fatal(err)
	}
	var doc ocli.CatalogDoc
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	sh, err := tl.shell()
	if err != nil {
		t.Fatal(err)
	}
	ops := sh.Runtime.Operations
	framework := ochazuke.NewRuntime().Operations
	n := 0
	for _, noun := range doc.Nouns {
		for _, v := range noun.Verbs {
			key := noun.Name + "." + v.Name
			if _, ok := framework[key]; ok {
				continue
			}
			n++
			if v.History == "" || ops[key].HistoryKind == "" || v.History != ops[key].HistoryKind {
				t.Errorf("catalog verb %s: history %q, declared %q", key, v.History, ops[key].HistoryKind)
			}
		}
	}
	if n != 11 {
		t.Fatalf("the catalog has %d o-proj verbs, want 11", n)
	}
	for key, want := range map[string]string{
		"project.list": ochazuke.HistoryFetch, "project.status": ochazuke.HistoryFetch, "template.list": ochazuke.HistoryFetch,
		"project.new": ochazuke.HistoryChange, "project.sync": ochazuke.HistoryChange, "project.link": ochazuke.HistoryChange,
	} {
		if got := ops[key].HistoryKind; got != want {
			t.Errorf("%s: %q, want %q", key, got, want)
		}
	}
}
