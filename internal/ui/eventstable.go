package ui

import (
	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
)

// The events table is the one rendering of an events feed on an owner page:
// the Events tab (with the Resource column and its filter links) and the details
// panel's Events view (without it). Its input, eventsTable and eventRow, is
// defined in platform.go with the partial; this file adds the owner pages' use.

// Empty texts of an events table.
const (
	noEvents      = "No events in the last hour or so."
	noEventsMatch = "No events match these filters."
)

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
