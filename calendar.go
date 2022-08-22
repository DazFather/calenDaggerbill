package main

import (
	"strings"
	"time"
)

type CalendarError string

const (
	INVALID_CALENDAR CalendarError = "Empty calendar, invitation might be expired"
	INVALID_EVENT    CalendarError = "This date is not avaiable anymore"
	ALREADY_JOINED   CalendarError = "Event already joined"
)

const DATETIME_FROMAT = "02/01/2006T15:04"

func (e CalendarError) Error() string {
	return string(e)
}

type Calendar struct {
	name        string
	description string
	invitation  string
	dates       map[string]*Event
}

type Date time.Time

func ParseDate(source string) *Date {
	switch strings.ToLower(source) {
	case "current", "today":
		d := Now()
		return &d
	case "tomorrow":
		d := Now().Skip(0, 0, 1)
		return &d
	}

	var date, err = time.Parse(DATETIME_FROMAT, source)
	if err == nil {
		d := Date(date)
		return &d
	}
	return nil
}

func Now() Date {
	return Date(time.Now())
}

func (d Date) Day() (day int, ofWeek int) {
	t := d.Time()
	return t.Day(), int(t.Weekday()) - 1
}

func (d Date) Week() int {
	return int(d.Time().Weekday())
}

func (d Date) Time() time.Time {
	return time.Time(d)
}

func (d Date) Format() string {
	return d.Time().Format(DATETIME_FROMAT)
}

func (d Date) Month() (month time.Month, maxDays int) {
	var t = d.Time()
	return t.Month(), t.AddDate(0, 1, -t.Day()).Day()
}

func (d Date) MonthStart() Date {
	var t = d.Time()
	return Date(t.AddDate(0, 0, -t.Day()+1))
}

func (d Date) MonthEnd() Date {
	var t = d.Time()
	return Date(t.AddDate(0, 1, -t.Day()+1))
}

func (d Date) Skip(years int, months int, days int) Date {
	return Date(d.Time().AddDate(years, months, days))
}

func (c *Calendar) addDate(date Date) {
	c.dates[date.Format()] = new(Event)
}

func (c *Calendar) joinDate(date Date, userID int64) error {
	if c == nil {
		return INVALID_CALENDAR
	}

	var event = c.dates[date.Format()]
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
