package mixer

import "fmt"

// uniqueName returns name if it is not yet in used, otherwise "<name> (2)",
// "<name> (3)" and so on: the first such variant that is absent from used.
// The returned name is added to used, so repeated calls with the same name
// continue the numbering.
//
// mihomo and sing-box reject the whole config on duplicate names
// (proxies[].name / outbounds[].tag), so mixed-in entries whose name is
// already taken (by the panel or by another mixed-in entry) must be renamed
// before being added.
func uniqueName(used map[string]bool, name string) string {
	if !used[name] {
		used[name] = true
		return name
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s (%d)", name, i)
		if !used[candidate] {
			used[candidate] = true
			return candidate
		}
	}
}
