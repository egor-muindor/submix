package extras

// ApplyRules runs an entry name through the subscription's rules. The entry
// passes if at least one rule matches; it receives the union of the tags of
// all matching rules (without duplicates, in order of appearance).
func ApplyRules(rules []Rule, name string) ([]string, bool) {
	var tags []string
	seen := map[string]bool{}

	for _, r := range rules {
		if r.Re == nil || !r.Re.MatchString(name) {
			continue
		}
		for _, tag := range r.Tags {
			if seen[tag] {
				continue
			}
			seen[tag] = true
			tags = append(tags, tag)
		}
	}
	return tags, len(tags) > 0
}
