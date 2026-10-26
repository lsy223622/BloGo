package site

import (
	"path"
	"sort"
	"strings"

	"github.com/lsy223622/BloGo/internal/content"
)

// WildcardAliases orders patterns by literal length, then lexically for ties.
func WildcardAliases(redirects map[string]string) []string {
	var patterns []string
	for source := range redirects {
		if strings.Contains(source, "*") {
			patterns = append(patterns, source)
		}
	}
	sort.Slice(patterns, func(i, j int) bool {
		a, b := patterns[i], patterns[j]
		la, lb := len(a)-strings.Count(a, "*"), len(b)-strings.Count(b, "*")
		if la != lb {
			return la > lb
		}
		return a < b
	})
	return patterns
}

func MatchWildcardAlias(pattern, requestPath string) bool {
	if content.PrivatePath(requestPath) || ReservedRoute(requestPath) {
		return false
	}
	matched, _ := path.Match(pattern, requestPath)
	return matched
}
