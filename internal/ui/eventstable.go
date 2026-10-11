package ui

import (
	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
)

// The events table is the one rendering of an events feed on an owner page:
// the Events tab (with the Resource column and its filter links) and the details
// panel's Events view (without it). Its input is the shape that
// align-platform-installed-catalog's design.md names for the shared partial.

// Empty texts of an events table.
const (
	noEvents      = "No events in the last hour or so."
	noEventsMatch = "No events match these filters."
)

// eventsTable is the input of the events-table partial.
type eventsTable struct {
	Problem *v1.Problem
	Rows    []eventRow
	// ShowResource adds the Resource column.
	ShowResource bool
	// Empty is the text when there is no row.
	Empty string
}

// eventRow is one folded event line.
type eventRow struct {
	v1.Event
	// Kind and Name are the regarded object's kind, without its group, and
	// its name.
	Kind, Name string
	// ResourceHref is the filter link of the resource cell, empty when the
	// cell is not a filter link.
	ResourceHref string
}

// eventRowOf makes the row of e; href is its resource filter link, or "".
// The note is shown as the read API served it, already capped: the page
// clips it no further.
func eventRowOf(e v1.Event, href string) eventRow {
	return eventRow{Event: e, Kind: e.Regarding.Kind, Name: e.Regarding.Name, ResourceHref: href}
}

// Table is the rows of a single object's feed, with no Resource column.
func (e eventsView) Table() eventsTable {
	rows := make([]eventRow, len(e.Items))
	for i := range e.Items {
		rows[i] = eventRowOf(e.Items[i], "")
	}
	return eventsTable{Problem: e.Problem, Rows: rows, Empty: noEvents}
}
