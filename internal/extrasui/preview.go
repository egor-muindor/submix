package extrasui

import (
	"regexp"
	"sort"
	"strconv"

	"github.com/egor-muindor/submix/internal/entry"
	"github.com/egor-muindor/submix/internal/extras"
)

// Preview computes from the draft everything the page shows: the validation
// error, the YAML, the rule results for every subscription, the conversion
// errors of static entries and the set of entries for a user with PanelTag.
// User entries come in the order production serves them: subscriptions by
// name (as Store.All sorts them), then static entries.
func Preview(req PreviewRequest) PreviewResponse {
	resp := PreviewResponse{
		Subscriptions: map[string]SubscriptionPreview{},
		StaticErrors:  make([]map[string]string, len(req.Config.StaticEntries)),
	}

	if text, err := RenderYAML(&req.Config); err != nil {
		resp.ValidationError = "render yaml: " + err.Error()
	} else {
		resp.YAML = text
		if _, err := extras.Parse([]byte(text)); err != nil {
			resp.ValidationError = err.Error()
		}
	}

	userTags := extras.TagsFor(req.Config.Users, req.PanelTag)
	resp.User = UserPreview{Tags: userTags, Entries: []UserEntry{}}

	kept := map[string][]entry.Entry{}
	for _, sub := range req.Config.Subscriptions {
		prev, k := previewSubscription(sub.Rules, req.Fetched[sub.Name].Entries)
		resp.Subscriptions[sub.Name] = prev
		kept[sub.Name] = k
	}
	names := make([]string, 0, len(kept))
	for name := range kept {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		for _, e := range extras.SelectEntries(kept[name], userTags) {
			resp.User.Entries = append(resp.User.Entries, UserEntry{Name: e.Name, Source: name, Tags: e.Tags})
		}
	}

	static := make([]entry.Entry, 0, len(req.Config.StaticEntries))
	for i, st := range req.Config.StaticEntries {
		e := entry.Entry{Name: st.Name, URI: st.URI, Tags: st.Tags}
		resp.StaticErrors[i] = ConversionErrors(e)
		static = append(static, e)
	}
	for _, e := range extras.SelectEntries(static, userTags) {
		resp.User.Entries = append(resp.User.Entries, UserEntry{Name: e.Name, Source: "static", Tags: e.Tags})
	}

	return resp
}

// compileRules compiles the rules one by one and collects errors by index:
// the same errors Validate would find, but reported per rule. An invalid
// regexp leaves Re == nil (ApplyRules skips those). A rule without tags is
// still compiled: the entries table shows that it matches but yields no tags.
func compileRules(rules []extras.Rule) ([]extras.Rule, map[string]string) {
	compiled := make([]extras.Rule, len(rules))
	errs := map[string]string{}
	for i, r := range rules {
		compiled[i] = extras.Rule{Match: r.Match, Tags: r.Tags}
		if r.Match == "" {
			errs[strconv.Itoa(i)] = "match is required"
			continue
		}
		re, err := regexp.Compile(r.Match)
		if err != nil {
			errs[strconv.Itoa(i)] = err.Error()
			continue
		}
		compiled[i].Re = re
		if len(r.Tags) == 0 {
			errs[strconv.Itoa(i)] = "tags are required"
		}
	}
	return compiled, errs
}

// previewSubscription applies the rules to the fetched entries. It returns the
// preview for the page and the kept entries with their tags, the candidates
// for resolving.
func previewSubscription(rules []extras.Rule, fetched []FetchedEntry) (SubscriptionPreview, []entry.Entry) {
	compiled, ruleErrors := compileRules(rules)
	prev := SubscriptionPreview{RuleErrors: ruleErrors, Entries: make([]EntryMatch, 0, len(fetched))}
	kept := []entry.Entry{}

	for _, fe := range fetched {
		// Two passes on purpose: Matched holds the indexes highlighted in the
		// table, Tags is the production truth via the same ApplyRules the
		// fetcher uses.
		matched := []int{}
		for i, r := range compiled {
			if r.Re != nil && r.Re.MatchString(fe.Name) {
				matched = append(matched, i)
			}
		}
		tags, ok := extras.ApplyRules(compiled, fe.Name)
		if tags == nil {
			tags = []string{}
		}
		prev.Entries = append(prev.Entries, EntryMatch{Name: fe.Name, Matched: matched, Tags: tags})
		if ok {
			kept = append(kept, entry.Entry{Name: fe.Name, URI: fe.URI, Tags: tags})
		}
	}
	return prev, kept
}
