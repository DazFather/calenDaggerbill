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
	notification bool
	name         string
	description  string
	invitation   string
	lastTimeUsed Date
	dates        map[string]*Event
}

type Date time.Time

func NewCalendar(name, description, invitation string) *Calendar {
	return &Calendar{
		name:         name,
		description:  description,
		notification: true,
		invitation:   invitation,
		lastTimeUsed: Now(),
	}
}

func (c Calendar) UnusedFor() time.Duration {
	return time.Since(c.lastTimeUsed.Time())
}

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

func (d Date) IsBefore(date Date) bool {
	return d.Time().Before(date.Time())
}

func (d Date) IsAfter(date Date) bool {
	return d.Time().After(date.Time())
}

func (d Date) When(do func()) {
	go func() {
		<-time.After(d.Time().Sub(time.Now()))
		do()
	}()
}

func (c *Calendar) addDate(date Date) (confirm bool) {
	c.lastTimeUsed = Now()
	if c.dates[date.Format()] == nil {
		c.dates[date.Format()] = new(Event)
		confirm = true
	}
	return
}

func (c *Calendar) removeDate(date Date) (deleted *Event) {
	c.lastTimeUsed = Now()
	key := date.Format()
	deleted = c.dates[key]
	if deleted != nil {
		delete(c.dates, key)
	}
	return
}

func (c *Calendar) joinDate(date Date, userID int64) error {
	c.lastTimeUsed = Now()
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

func (c Calendar) CurrentAttendee(forDate Date) []int64 {
	return c.dates[forDate.Format()].attendee
}

func (c Calendar) AllCurrentAttendee() []int64 {
	var exists = make(map[int64]bool)
	for _, event := range c.dates {
		for _, userID := range event.attendee {
			exists[userID] = true
		}
	}

	var attendee = make([]int64, len(exists))
	i := 0
	for userID := range exists {
		attendee[i] = userID
		i++
	}
	return attendee
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
