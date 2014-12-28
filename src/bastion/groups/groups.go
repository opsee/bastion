package groups

import (
		"fmt"
)

type Groups struct {
	lookup 		map[string]*Group
}

func Start() *Groups {
	groups := &Groups{lookup: make(map[string] *Group)}
	return groups
}

func (g *Groups) AddGroup(groupName string) (*Group) {
	group, ok := g.lookup[groupName]
	if ok {
		return group
	}
	group = startGroup(groupName)
	g.lookup[groupName] = group
	return group
}