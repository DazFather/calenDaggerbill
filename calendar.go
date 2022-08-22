package main

import "time"

type CalendarError string

const (
	INVALID_CALENDAR CalendarError = "Empty calendar, invitation might be expired"
	INVALID_EVENT    CalendarError = "This date is not avaiable anymore"
	ALREADY_JOINED   CalendarError = "Event already joined"
)

func (e CalendarError) Error() string {
	return string(e)
}

type Calendar struct {
	name        string
	description string
	invitation  string
	dates       map[string]*Event
}

func (c *Calendar) addDate(date time.Time) {
	c.dates[formatDate(date)] = new(Event)
}

func (c *Calendar) joinDate(date time.Time, userID int64) error {
	if c == nil {
		return INVALID_CALENDAR
	}

	var event = c.dates[formatDate(date)]
	if event == nil {
		return INVALID_EVENT
	}
	if event.hasJoined(userID) {
		return ALREADY_JOINED
	}

	event.join(userID)
	return nil
}

type Event struct {
	attendee []int64
}

func (e *Event) join(userID int64) {
	e.attendee = append(e.attendee, userID)
}

func (e Event) countAttendee() int {
	return len(e.attendee)
}

func (e Event) hasJoined(userID int64) bool {
	for _, guestID := range e.attendee {
		if guestID == userID {
			return true
		}
	}
	return false
}
