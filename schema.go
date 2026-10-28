package appkit

import (
	"webtyp.com/ddl"
	"webtyp.com/erd"
	"webtyp.com/model"
)

// groupsOf turns captured tables into one diagram group per migration step, in step order.
func groupsOf(tables []ddl.Table) []erd.Group {
	var groups []erd.Group
	for _, t := range tables {
		if n := len(groups); n > 0 && groups[n-1].Name == t.Step {
			groups[n-1].Models = append(groups[n-1].Models, t.Model)
			continue
		}
		groups = append(groups, erd.Group{Name: t.Step, Models: []model.Model{t.Model}})
	}
	return groups
}
