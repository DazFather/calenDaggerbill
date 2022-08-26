package main

import (
	"strings"
	"time"
)

const UNUSED_TIME = time.Hour * 24 * 30 * 6

/* --- CALENDAR ERROR --- */

type CalendarError string

func (e CalendarError) Error() string {
	return string(e)
}

const (
	INVALID_CALENDAR CalendarError = "Empty calendar, invitation might be expired"
	INVALID_EVENT    CalendarError = "This date is not avaiable anymore"
	ALREADY_JOINED   CalendarError = "Event already joined"

	INVALID_DATE CalendarError = ""
)

/* --- CALENDAR --- */

type Calendar struct {
	notification toggler
	name         string
	description  string
	invitation   string
	lastTimeUsed Date
	dates        map[FormattedDate]*Event
}

func NewCalendar(name, description, invitation string) *Calendar {
	return &Calendar{
		name:         name,
		description:  description,
		notification: true,
		invitation:   invitation,
		lastTimeUsed: Now(),
	}
}

func (c *Calendar) addDate(date FormattedDate) (confirm bool) {
	c.lastTimeUsed = Now()

	if c.dates[date] == nil {
		c.dates[date] = new(Event)
		confirm = true
	}
	return
}

func (c *Calendar) removeDate(date FormattedDate) (deleted *Event) {
	c.lastTimeUsed = Now()

	deleted = c.dates[date]
	if deleted != nil {
		delete(c.dates, date)
	}
	return
}

func (c *Calendar) joinDate(date FormattedDate, userID int64) error {
	if c == nil {
		return INVALID_CALENDAR
	}
	c.lastTimeUsed = Now()

	var event = c.dates[date]
	if event == nil {
		return INVALID_EVENT
	}
	if event.hasJoined(userID) {
		return ALREADY_JOINED
	}

	event.join(userID)
	return nil
}

func (c Calendar) CountAttendee(forDate FormattedDate) int {
	return len(c.CurrentAttendee(forDate))
}

func (c Calendar) CurrentAttendee(forDate FormattedDate) []int64 {
	return c.dates[forDate].attendee
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

func (c Calendar) IsUnused() bool {
	return len(c.dates) == 0 && time.Since(c.lastTimeUsed.Time) >= UNUSED_TIME
}

/* --- EVENT --- */

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

/* --- FORMATTED DATE --- */

type FormattedDate string

func Format(t time.Time) FormattedDate {
	return FormattedDate(t.Format(DATETIME_FROMAT))
}

func Today() FormattedDate {
	return Format(time.Now())
}

func (f FormattedDate) Beautify() string {
	return strings.Replace(string(f), "T", " ", 1)
}

/* --- DATE --- */

type Date struct {
	beautified string
	formatted  FormattedDate
	time.Time
}

const DATETIME_FROMAT = "02/01/2006T15:04"

func Parse(t time.Time) Date {
	f := Format(t)
	return Date{f.Beautify(), f, t}
}

func Now() Date {
	return Parse(time.Now())
}

func ParseDate(source string) (d Date, err error) {
	switch strings.ToLower(source) {
	case "current", "today":
		return Now(), nil
	case "tomorrow":
		return Now().Skip(0, 0, 1), nil
	}

	date, err := time.Parse(DATETIME_FROMAT, source)
	if err == nil {
		d = Parse(date)
	}
	return
}

func (d Date) Formatted() FormattedDate {
	return d.formatted
}

func (d Date) String() string {
	return d.beautified
}

func (d Date) Week() int {
	return int(d.Weekday())
}

func (d Date) MonthInfo() (month time.Month, maxDays int) {
	return d.Month(), d.MonthEnd().Day()
}

func (d Date) MonthStart() Date {
	return d.Skip(0, 0, -d.Day()+1)
}

func (d Date) MonthEnd() Date {
	return d.Skip(0, 1, -d.Day()+1)
}

func (d Date) Skip(years int, months int, days int) Date {
	return Parse(d.AddDate(years, months, days))
}

func (d Date) IsBefore(date Date) bool {
	return d.Before(date.Time)
}

func (d Date) IsAfter(date Date) bool {
	return d.After(date.Time)
}

func (d Date) WhenOccurrs(do func()) {
	go func() {
		<-time.After(d.Sub(time.Now()))
		do()
	}()
}

/* --- TOGGLER --- */

type toggler bool

func (t toggler) String() string {
	if t {
		return "on"
	}
	return "off"
}

func ParseToggler(of string) (t *toggler) {
	switch of {
	case "on":
		t = new(toggler)
		*t = true
	case "off":
		t = new(toggler)
		*t = false
	}
	return
}

func (t *toggler) Toggle() *toggler {
	if t != nil {
		*t = !*t
	}
	return t
}

func Repeat(every time.Duration, do func()) {
	c := time.Tick(UNUSED_TIME)
	for _ = range c {
		do()
	}
}
