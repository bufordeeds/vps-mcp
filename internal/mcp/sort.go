package mcp

import "sort"

func sortTools(t []Tool) {
	sort.Slice(t, func(i, j int) bool { return t[i].Name() < t[j].Name() })
}
