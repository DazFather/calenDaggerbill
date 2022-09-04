package main

import (
	"fmt"
	"strings"

	"github.com/DazFather/parrbot/message"
	"github.com/DazFather/parrbot/robot"
	"github.com/DazFather/parrbot/tgui"
)

/* --- EMOJI ICONS --- */
type icon string

const (
	BLOCK     icon = "🚫"
	CLOSE     icon = "❎"
	CONFIRM   icon = "✅"
	DONE      icon = "✔️"
	CANCEL    icon = "❌"
	NOTIF_ON  icon = "🔔"
	NOTIF_OFF icon = "🔕"
	BACK      icon = "🔙"
	REFRESH   icon = "🔄"
	LOGO      icon = "🐦"
	CALENDAR  icon = "📅"
	PEOPLE    icon = "👥"
)

func (emoji icon) Text(s string) string {
	return fmt.Sprint(emoji, " ", s)
}

/* --- FREQUENT BUTTONS --- */
var (
	BTN_CLOSE   = closeCaller(CLOSE.Text("Close"), "")
	BTN_CANCEL  = closeCaller(CANCEL.Text("Cancel"), DONE.Text("Operation cancelled"))
	BTN_DELETED = closeCaller(CLOSE.Text("Close"), DONE.Text("Deleted"))
)

/* --- MESSAGE BUILDERS --- */

func buildCalendarMessage(date Date, text string) message.Text {
	var (
		month     = date.Month()
		monthDays = date.MonthEnd().Day()
		weekday   = date.MonthStart().Week()
		buttons   = make([]tgui.InlineButton, monthDays+weekday)
		row       []tgui.InlineButton
		now       = Now()
	)

	for i := 0; i < weekday; i++ {
		buttons[i] = alertCaller(BLOCK, "", "Cannot create an event in this day")
	}

	row = buttons[weekday:]
	date = date.MonthStart()
	for i := range row {
		var (
			label string = fmt.Sprint(i + 1)
			day   Date   = date.Skip(0, 0, i)
		)

		if day.IsBefore(now) {
			row[i] = alertCaller(BLOCK, "", "Cannot create an event in this day")
		} else {
			row[i] = tgui.InlineCaller(label, "/publish", string(day.Formatted()), "add")
		}
	}

	keyboard := append([][]tgui.InlineButton{{
		tgui.InlineCaller("⏮", "/publish", string(date.Skip(0, -1, 0).Formatted()), "refresh"),
		alertCaller(BLOCK, month.String(), "This feature is not yet avaiable"),
		tgui.InlineCaller("⏭", "/publish", string(date.Skip(0, 1, 0).Formatted()), "refresh"),
	}}, tgui.Arrange(7, buttons...)...)

	row = keyboard[len(keyboard)-1]
	for i := len(row); i < 7; i++ {
		row = append(row, alertCaller(BLOCK, "", "Cannot create an event in this day"))
	}
	keyboard[len(keyboard)-1] = row

	return genDefaultMessage(CALENDAR, text+month.String(), append(keyboard, []tgui.InlineButton{
		BTN_CANCEL,
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
				caption = fmt.Sprint(DONE, " ", caption, " - ", PEOPLE, n-1, " + 1 (You)")
			} else {
				caption += fmt.Sprint("- ", PEOPLE, n)
			}
		}
		kbd[i] = tgui.Wrap(tgui.InlineCaller(caption, "/join", c.invitation, string(date)))
		i++
	}
	kbd[i] = []tgui.InlineButton{
		tgui.InlineCaller(REFRESH.Text("Refresh"), "/start", c.invitation),
		BTN_CLOSE,
	}

	return genDefaultMessage(
		icon("🛎"),
		"<b>"+c.name+"</b>\n"+c.description+"\n\n<i>Tap one (or more) of following dates to join</i>",
		kbd...,
	)
}

/*
func buildEditorMessage(c Calendar) message.Text {
	var kbd = make([][]tgui.InlineButton, len(c.dates))

	i := 0
	for date, _ := range c.dates {
		kbd[i] = []tgui.InlineButton{
			tgui.InlineCaller(date, "/alert", "ciao"),
			tgui.InlineCaller("", "/edit", "notification", "toggle"),
			tgui.InlineCaller("🗑", "/delete", date),
		}
		i++
	}

	return message.Text{"", nil}
}*/

func buildErrorMessage(text string) message.Text {
	return genDefaultMessage(BLOCK, text, tgui.Wrap(BTN_CLOSE))
}

func genDefaultEditOpt(rows ...[]tgui.InlineButton) *tgui.EditOptions {
	opt := tgui.ParseModeOpt(nil, "HTML")
	if len(rows) > 0 {
		tgui.InlineKbdOpt(opt, rows)
	}
	return opt
}

func genDefaultMessage(emoji icon, text string, rows ...[]tgui.InlineButton) message.Text {
	return message.Text{emoji.Text(text), tgui.ToMessageOptions(genDefaultEditOpt(rows...))}
}

func sendNotification(chatID int64, text string) error {
	_, err := genDefaultMessage(NOTIF_ON, text, []tgui.InlineButton{
		BTN_DELETED,
		tgui.InlineCaller(NOTIF_OFF.Text("Turn off notifications"), "/edit", "notification", "off"),
	}).Send(chatID)

	return err
}

/* --- TOAST NOTIFICATION -- */

const MAX_CACHE_TIME = 3600

var (
	alertHandler, alertCaller = alerter("/alert")
	closeHandler, closeCaller = closer("/close")
)

func Notify(callback *message.CallbackQuery, emoji icon, text string) {
	if callback == nil {
		return
	}

	callback.AnswerToast(emoji.Text(text), MAX_CACHE_TIME)
}

func Collapse(callback *message.CallbackQuery, emoji icon, message string) {
	if message != "" {
		Notify(callback, emoji, message)
	} else if callback == nil {
		return
	}

	callback.Answer(nil)
	callback.Delete()
}

func alerter(trigger string) (command robot.Command, caller func(emoji icon, label, text string) tgui.InlineButton) {
	command = robot.Command{
		Trigger: trigger,
		ReplyAt: message.CALLBACK_QUERY,
		CallFunc: func(bot *robot.Bot, update *message.Update) message.Any {
			var callback = update.CallbackQuery
			Notify(callback, icon(""), strings.TrimPrefix(callback.Data, trigger))
			return nil
		},
	}

	caller = func(emoji icon, label, text string) tgui.InlineButton {
		return tgui.InlineCaller(emoji.Text(label), trigger, emoji.Text(text))
	}

	return
}

func closer(trigger string) (command robot.Command, caller func(label, text string) tgui.InlineButton) {
	command = tgui.Closer(trigger, false)

	caller = func(label, text string) tgui.InlineButton {
		if text == "" {
			return tgui.InlineCaller(label, trigger)
		}
		return tgui.InlineCaller(label, trigger, text)
	}

	return
}
