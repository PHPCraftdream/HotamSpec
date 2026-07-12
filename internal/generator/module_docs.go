package generator

import "strings"

type moduleEntry struct {
	Mod   string
	Label string
}

var ModuleOrder = []moduleEntry{
	{Mod: "__init__", Label: "Methodology overview + the closed loop"},
	{Mod: "stakeholder", Label: "§Stakeholder — owners and stewards"},
	{Mod: "axis", Label: "§Axis — controlled vocabulary of tension dimensions"},
	{Mod: "assumption", Label: "§Assumption — beliefs with a lifecycle"},
	{Mod: "requirement", Label: "§Requirement — the requirement node"},
	{Mod: "conflict", Label: "§Conflict — the connector node"},
	{Mod: "graph", Label: "§Graph — the store, the loader, and traversal"},
	{Mod: "lifecycle", Label: "§Lifecycle — the generic state-machine keystone"},
	{Mod: "operator", Label: "§Operator — the acting facet of a Stakeholder"},
	{Mod: "process", Label: "§Process / §Goal — behavioral aspect (M12) and Goal type (M19)"},
	{Mod: "invariants", Label: "§Invariants — structural form"},
}

func ModuleDocstring(mod string) string {
	if s, ok := moduleDocText[mod]; ok {
		return strings.TrimRight(s, " \t\r\n")
	}
	return ""
}
