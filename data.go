package main

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/NicoNex/echotron/v3"
)

const DEFAULT_UNUSED_TIME = time.Hour * 24 * 30 * 6

var organizers = map[int64]*Calendar{}

func CalendarOf(userID int64) *Calendar {
	return organizers[userID]
}

func ClearUnusedCalendars(every time.Duration) {
	go Repeat(every, func() {
		for userID, calendar := range organizers {
			if calendar.IsUnused(every) {
				delete(organizers, userID)
			}
		}
	})
}

func AddToCalendar(user echotron.User, dates ...Date) *Calendar {
	var calendar *Calendar = organizers[user.ID]
	if calendar == nil {
		calendar = NewCalendar(
			user.FirstName+" calendar",
			user.FirstName+" personal event",
			strconv.Itoa(int(user.ID)),
		)
		calendar.dates = make(map[FormattedDate]*Event, len(dates))
		organizers[user.ID] = calendar
	}

	for _, date := range dates {
		timestamp := date.Formatted()
		if !calendar.addDate(timestamp) {
			continue
		}

		date.Skip(0, 0, -7).WhenOccurrs(
			remind(calendar, timestamp, fmt.Sprint("Don't forget the ", calendar.name, ", is cooming soon: ", date)),
		)

		date.Skip(0, 0, -1).WhenOccurrs(
			remind(calendar, timestamp, fmt.Sprint("a Tomorrow ", date, ", there will be ", calendar.name, " waiting for you!")),
		)

		date.WhenOccurrs(func() {
			calendar.removeDate(timestamp)
		})
	}

	return calendar
}

func remind(calendar *Calendar, date FormattedDate, text string) (reminder func()) {
	return func() {
		if calendar == nil || !calendar.notification {
			return
		}

		for userID := range calendar.CurrentAttendee(date) {
			genDefaultMessage(NOTIF_ON, text).Send(int64(userID))
		}
	}
}

func JoinEvent(user echotron.User, invitation, rawDate string) (calendar *Calendar, err error) {
	var (
		timestamp FormattedDate
		ownerID   *int64 = retreiveOwner(invitation)
	)

	if ownerID == nil {
		return nil, errors.New("Invalid invitation")
	}

	if date, err := ParseDate(rawDate); err == nil {
		calendar = organizers[*ownerID]
		timestamp = date.Formatted()
		err = calendar.joinDate(timestamp, user.ID)
	}
	if err != nil {
		return
	}

	if calendar.notification {
		name := user.Username
		if name == "" {
			name = user.FirstName
		} else {
			name = "@" + name
		}
		count := "+ 1"
		if tot := calendar.CountAttendee(timestamp) - 1; tot > 0 {
			count = fmt.Sprint(tot, " ", count)
		}
		sendNotification(*ownerID, fmt.Sprint("<b>", count, "</b>: ", name, " joined your event in date: ", timestamp.Beautify()))
	}

	return
}

func GetShareLink(botUsername string, c Calendar) string {
	if botUsername == "" {
		return "/start " + c.invitation
	}
	return "t.me/" + botUsername + "?start=" + c.invitation
}

func retreiveOwner(invitation string) *int64 {
	var rawID, err = strconv.Atoi(invitation)
	if err != nil {
		return nil
	}

	userID := int64(rawID)
	return &userID
}

func retreiveCalendar(invitation string) *Calendar {
	if userID := retreiveOwner(invitation); userID != nil {
		return organizers[*userID]
	}
	return nil
}
