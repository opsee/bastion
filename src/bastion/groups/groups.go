package groups

import (
		"fmt"
)

type GroupType int

const (
	SecurityGroup GroupType = iota
	AutoScaleGroup GroupType = iota
)

type Groups struct {
	lookup 		map[string]Group
}

type Group struct {
	groupName 	string
	groupID 	string
	groupType	GroupType
	lookup		map[string]Server
}

func Start() *Groups {
	groups := &Groups{lookup: make(map[string] *Group)}
	return groups
}

func (g *Groups) AddGroup(groupName string) *Group {
	group, ok := g.lookup[groupName]
	if ok {
		return group
	}
	group = newGroup(groupName)
	g.lookup[groupName] = group
	return group
}

func newGroupFromSG(sg *ec2.SecurityGroup) *Group {
	return &Group{
		groupName : sg.GroupName,
		groupID : sg.GroupID,
		groupType : SecurityGroup,
		lookup : make(map[string]*Server)}
}

func newGroup(groupName, groupID string, groupType GroupType) *Group {
	group = &Group{
		groupName : groupName,
		groupID : groupID,
		groupType : groupType,
		lookup : make(map[string] *Server)}
}

func (g *Group) CheckGroupMembership() {

}