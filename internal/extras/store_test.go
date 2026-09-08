package extras_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egor-muindor/submix/internal/entry"
	"github.com/egor-muindor/submix/internal/extras"
)

func TestStoreSetAndAll(t *testing.T) {
	dir := t.TempDir()
	s, err := extras.NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	s.Set("provider-a", []entry.Entry{{Name: "A", URI: "vless://u@h:443#A", Tags: []string{"de"}}})
	s.Set("provider-b", []entry.Entry{{Name: "B", URI: "ss://m:p@h:8388#B", Tags: []string{"bulk"}}})

	all := s.All()
	if len(all) != 2 {
		t.Fatalf("all = %#v", all)
	}

	// A repeated Set replaces the subscription's entries rather than appending.
	s.Set("provider-a", []entry.Entry{{Name: "A2", URI: "vless://u@h2:443#A2", Tags: []string{"de"}}})
	all = s.All()
	if len(all) != 2 {
		t.Fatalf("all after replace = %#v", all)
	}
}

func TestStorePersistsAndRestores(t *testing.T) {
	dir := t.TempDir()

	s1, err := extras.NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	s1.Set("provider-a", []entry.Entry{{Name: "A", URI: "vless://u@h:443#A", Tags: []string{"de"}}})

	files, _ := os.ReadDir(dir)
	if len(files) != 1 {
		t.Fatalf("cache files = %#v", files)
	}

	s2, err := extras.NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore #2: %v", err)
	}
	if err := s2.Restore([]string{"provider-a", "provider-b"}); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	all := s2.All()
	if len(all) != 1 || all[0].Name != "A" {
		t.Fatalf("restored = %#v", all)
	}
}

func TestStoreRestoreIgnoresCorruptFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "provider-a.json"), []byte("{{{"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	s, err := extras.NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if err := s.Restore([]string{"provider-a"}); err != nil {
		t.Fatalf("Restore must not fail on corrupt cache: %v", err)
	}
	if len(s.All()) != 0 {
		t.Fatalf("all = %#v", s.All())
	}
}
