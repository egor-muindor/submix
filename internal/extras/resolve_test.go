package extras_test

import (
	"reflect"
	"testing"

	"github.com/egor-muindor/submix/internal/entry"
	"github.com/egor-muindor/submix/internal/extras"
)

func TestTagsForMergesDefaultsAndMapping(t *testing.T) {
	users := extras.Users{
		DefaultTags: []string{"bulk", "de"},
		ByPanelTag:  map[string][]string{"PREMIUM": {"de", "premium"}},
	}

	if got, want := extras.TagsFor(users, "PREMIUM"), []string{"bulk", "de", "premium"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("TagsFor(PREMIUM) = %v, want %v", got, want)
	}
	if got, want := extras.TagsFor(users, "UNKNOWN"), []string{"bulk", "de"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("TagsFor(UNKNOWN) = %v, want %v", got, want)
	}
	if got := extras.TagsFor(extras.Users{}, ""); got == nil || len(got) != 0 {
		t.Fatalf("TagsFor(empty) = %#v, want empty non-nil slice", got)
	}
}

func TestSelectEntriesByTagIntersection(t *testing.T) {
	candidates := []entry.Entry{
		{Name: "DE-1", Tags: []string{"de"}},
		{Name: "GE-1", Tags: []string{"ge"}},
		{Name: "X", Tags: []string{"de", "ge"}},
	}

	got := extras.SelectEntries(candidates, []string{"ge"})
	if len(got) != 2 || got[0].Name != "GE-1" || got[1].Name != "X" {
		t.Fatalf("SelectEntries(ge) = %#v", got)
	}
	if got := extras.SelectEntries(candidates, nil); got == nil || len(got) != 0 {
		t.Fatalf("SelectEntries(no tags) = %#v, want empty non-nil slice", got)
	}
}
