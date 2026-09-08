package extras

import "github.com/egor-muindor/submix/internal/entry"

// TagsFor expands a user's panel tag into the set of mix tags: default_tags
// plus the by_panel_tag mapping, without duplicates, in order of appearance.
// If there are no tags, it returns an empty (not nil) slice.
func TagsFor(users Users, panelTag string) []string {
	seen := map[string]bool{}
	tags := []string{}

	add := func(list []string) {
		for _, tag := range list {
			if seen[tag] {
				continue
			}
			seen[tag] = true
			tags = append(tags, tag)
		}
	}

	add(users.DefaultTags)
	if panelTag != "" {
		add(users.ByPanelTag[panelTag])
	}
	return tags
}

// SelectEntries picks the entries whose tags intersect the user's tags.
// Candidate order is preserved. An empty tag set yields an empty slice.
func SelectEntries(candidates []entry.Entry, userTags []string) []entry.Entry {
	if len(userTags) == 0 {
		return []entry.Entry{}
	}
	result := make([]entry.Entry, 0, len(candidates))
	for _, e := range candidates {
		if e.MatchesAny(userTags) {
			result = append(result, e)
		}
	}
	return result
}
