package extras_test

import (
	"testing"

	"github.com/egor-muindor/submix/internal/extras"
)

func TestApplyRules(t *testing.T) {
	cfg, err := extras.Load("testdata/valid.yaml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	sub := cfg.Subscriptions[0] // provider-a: Germany|DE -> de; Tbilisi|🇬🇪 -> ge, premium

	t.Run("single rule match", func(t *testing.T) {
		tags, ok := extras.ApplyRules(sub.Rules, "🇩🇪 Germany-1")
		if !ok {
			t.Fatal("want match")
		}
		if len(tags) != 1 || tags[0] != "de" {
			t.Fatalf("tags = %#v", tags)
		}
	})

	t.Run("no match is dropped", func(t *testing.T) {
		if _, ok := extras.ApplyRules(sub.Rules, "🇫🇷 Paris-2"); ok {
			t.Fatal("must not match")
		}
	})

	t.Run("multiple rules union tags without duplicates", func(t *testing.T) {
		rules := append([]extras.Rule{}, sub.Rules...)
		tags, ok := extras.ApplyRules(rules, "DE 🇬🇪 Tbilisi")
		if !ok {
			t.Fatal("want match")
		}
		if len(tags) != 3 {
			t.Fatalf("tags = %#v, want de+ge+premium", tags)
		}
		seen := map[string]bool{}
		for _, tag := range tags {
			if seen[tag] {
				t.Fatalf("duplicate tag %q in %#v", tag, tags)
			}
			seen[tag] = true
		}
	})
}
