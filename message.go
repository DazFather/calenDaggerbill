package main

import (
	"fmt"

	"github.com/DazFather/parrbot/message"
	"github.com/DazFather/parrbot/tgui"
)

func buildCalendarMessage(date Date, text string) message.Text {
	var (
		month   = date.Month()
		weekday = date.MonthStart().Week()
		buttons = make([]tgui.InlineButton, date.MonthEnd().Day()+weekday)
		row     []tgui.InlineButton
		now     = Now()
	)

	for i := 0; i < weekday; i++ {
		buttons[i] = tgui.InlineCaller("🚫", "/alert", "❌ Cannot create an event in this day")
	}

	row = buttons[weekday:]
	date = date.MonthStart()
	for i := range row {
		var (
			label string = string(i + 1)
			day   Date   = date.Skip(0, 0, i)
		)

		if day.IsBefore(now) {
			row[i] = tgui.InlineCaller("🚫"+label, "/alert", "❌ Cannot create an event in this day")
		} else {
			row[i] = tgui.InlineCaller(label, "/publish", string(day.Formatted()), "add")
		}
	}

	keyboard := append([][]tgui.InlineButton{{
		tgui.InlineCaller("⏮", "/publish", string(date.Skip(0, -1, 0).Formatted()), "refresh"),
		tgui.InlineCaller(month.String(), "/alert", "This feature is not yet avaiable"),
		tgui.InlineCaller("⏭", "/publish", string(date.Skip(0, 1, 0).Formatted()), "refresh"),
	}}, tgui.Arrange(7, buttons...)...)

	row = keyboard[len(keyboard)-1]
	for i := len(row); i < 7; i++ {
		row = append(row, tgui.InlineCaller("🚫", "/alert", "❌ Cannot create an event in this day"))
	}
	keyboard[len(keyboard)-1] = row

	return genDefaultMessage(text+month.String(), append(keyboard, []tgui.InlineButton{
		tgui.InlineCaller("❌ Cancel", "/close", "✔️ Operation cancelled"),
		tgui.InlineCaller("🔄Refresh", "/publish", string(date.Formatted()), "refresh"),
	})...)
}

func buildDateListMessage(c Calendar, userID int64) message.Text {
	var kbd = make([][]tgui.InlineButton, len(c.dates)+1)

	i := 0
	for date, event := range c.dates {
		var caption string = date.Beautify()
		if n := event.countAttendee(); n > 0 {
			if event.hasJoined(userID) {
				caption = fmt.Sprint("✅ ", caption, " - 👥", n-1, " + 1 (You)")
			} else {
				caption += fmt.Sprint("- 👥", n)
			}
		}
		kbd[i] = tgui.Wrap(tgui.InlineCaller(caption, "/join", c.invitation, string(date)))
		i++
	}
	kbd[i] = []tgui.InlineButton{
		tgui.InlineCaller("🔄Refresh", "/start", c.invitation),
		tgui.InlineCaller("❎ Close", "/close"),
	}

	return genDefaultMessage(
		"🛎 <b>"+c.name+"</b>\n"+c.description+"\n\n<i>Tap one (or more) of following dates to join</i>",
		kbd...,
	)
}

func buildErrorMessage(text string) message.Text {
	return genDefaultMessage("🚫 "+text, tgui.Wrap(tgui.InlineCaller("❎ Close", "/close")))
}

func genDefaultEditOpt(rows ...[]tgui.InlineButton) *tgui.EditOptions {
	opt := tgui.ParseModeOpt(nil, "HTML")
	if len(rows) > 0 {
		tgui.InlineKbdOpt(opt, rows)
	}
	return opt
}

func genDefaultMessage(text string, rows ...[]tgui.InlineButton) message.Text {
	return message.Text{text, tgui.ToMessageOptions(genDefaultEditOpt(rows...))}
}

func sendNotification(chatID int64, text string) error {
	_, err := genDefaultMessage("🔔"+text, []tgui.InlineButton{
		tgui.InlineCaller("❎ Close", "/close", "✔️ Notification deleted"),
		tgui.InlineCaller("🔕 Turn off notifications", "/edit", "notification", "off"),
	}).Send(chatID)

	return err
}
