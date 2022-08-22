package main

import (
	"strconv"
	"strings"

	"github.com/DazFather/parrbot/message"
	"github.com/DazFather/parrbot/robot"
	"github.com/DazFather/parrbot/tgui"

	"github.com/NicoNex/echotron/v3"
)

func beautifyDate(date string) string {
	return strings.Replace(date, "T", " ", 1)
}

var organizers = map[int64]*Calendar{}

func main() {
	robot.Start(startHandler, joinHandler, publishHandler, closeHandler, alertHandler)
}

var startHandler = robot.Command{
	Description: "Start the bot",
	Trigger:     "/start",
	ReplyAt:     message.MESSAGE + message.CALLBACK_QUERY,
	CallFunc: func(bot *robot.Bot, update *message.Update) message.Any {
		var _, payload = extractCommand(update)
		if len(payload) == 0 {
			return message.Text{"Welcome!", nil}
		}

		if calendar := retreiveCalendar(payload[0]); calendar != nil {
			return buildDateListMessage(*calendar, bot.ChatID)
		}
		return message.Text{"Invalid invitation link", nil}
	},
}

var joinHandler = robot.Command{
	Trigger: "/join",
	ReplyAt: message.CALLBACK_QUERY,
	CallFunc: func(bot *robot.Bot, update *message.Update) message.Any {
		var (
			date     string
			ownerID  int64
			calendar *Calendar
		)

		if _, payload := extractCommand(update); len(payload) != 2 {
			return message.Text{"Invalid joining: " + update.CallbackQuery.Data, nil}
		} else if userID := retreiveOwner(payload[0]); userID == nil {
			return message.Text{"Invalid invitation: " + payload[0], nil}
		} else if d := ParseDate(payload[1]); d == nil {
			return message.Text{"Invalid date: " + payload[1], nil}
		} else if err := organizers[*userID].joinDate(*d, bot.ChatID); err != nil {
			return message.Text{"🚫 " + err.Error(), nil}
		} else {
			date, ownerID, calendar = beautifyDate(payload[1]), *userID, organizers[*userID]
		}

		collapse(update.CallbackQuery, "✅ You joined this event")
		name := update.CallbackQuery.From.Username
		message.Text{"+ 1: " + name + " joined your event in date: " + date, nil}.Send(ownerID)
		return buildDateListMessage(*calendar, bot.ChatID)
	},
}

var publishHandler = robot.Command{
	Description: "Publish a new event",
	Trigger:     "/publish",
	ReplyAt:     message.MESSAGE + message.CALLBACK_QUERY,
	CallFunc: func(bot *robot.Bot, update *message.Update) message.Any {
		var msg message.Any

		switch _, payload := extractCommand(update); len(payload) {
		case 0:
			if update.CallbackQuery != nil {
				update.FromMessage().Delete()
				return message.Text{"No given payload", nil}
			}
			msg = buildCalendarMessage(Now(), "🗓 Select a day from the calendar: ")
		case 1:
			var selected Date
			if d := ParseDate(payload[0]); d == nil {
				update.FromMessage().Delete()
				return message.Text{"Invaid date: " + payload[0], nil}
			} else {
				selected = *d
			}

			msg = buildCalendarMessage(selected, "🗓 Select a day from the calendar: ")
		case 2:
			var date Date
			if d := ParseDate(payload[0]); d == nil {
				update.FromMessage().Delete()
				return message.Text{"Invaid date: " + payload[0], nil}
			} else {
				date = *d
			}

			if payload[1] == "refresh" {
				update.FromMessage().Delete()
				return buildCalendarMessage(date, "🗓 Select a day from the calendar: ")
			}

			var (
				hasCalendar bool = organizers[bot.ChatID] != nil
				calendar         = AddToCalendar(*update.CallbackQuery.From, date)
			)
			notify(update.CallbackQuery, "📅 Date: "+beautifyDate(date.Format())+" added to your calendar ✔️")
			if !hasCalendar {
				msg = message.Text{
					strings.Join([]string{
						"✔️ <i>You calendar has been created</i>",
						"Use again the previous message to add a new avaiable date",
						"Share this to make people join: <code>/start " + calendar.invitation + "</code>",
					}, "\n"),
					&echotron.MessageOptions{ParseMode: "HTML"},
				}
			}
		}
		return msg
	},
}

var closeHandler = robot.Command{
	Trigger: "/close",
	ReplyAt: message.CALLBACK_QUERY,
	CallFunc: func(bot *robot.Bot, update *message.Update) message.Any {
		collapse(update.CallbackQuery, strings.TrimPrefix(update.CallbackQuery.Data, "/close"))
		return nil
	},
}

var alertHandler = robot.Command{
	Trigger: "/alert",
	ReplyAt: message.CALLBACK_QUERY,
	CallFunc: func(bot *robot.Bot, update *message.Update) message.Any {
		notify(update.CallbackQuery, strings.TrimPrefix(update.CallbackQuery.Data, "/alert"))
		return nil
	},
}

func notify(callback *message.CallbackQuery, text string) {
	callback.Answer(&echotron.CallbackQueryOptions{
		Text:      text,
		CacheTime: 3600,
	})
}

func AddToCalendar(user echotron.User, dates ...Date) *Calendar {
	var calendar *Calendar = organizers[user.ID]
	if calendar == nil {
		calendar = &Calendar{
			name:        user.FirstName + " calendar",
			description: user.FirstName + " personal event",
			invitation:  strconv.Itoa(int(user.ID)),
			dates:       make(map[string]*Event),
		}
		organizers[user.ID] = calendar
	}

	for _, date := range dates {
		calendar.addDate(date)
	}

	return calendar
}

func collapse(callback *message.CallbackQuery, message string) {
	if callback == nil {
		return
	}

	var opt *echotron.CallbackQueryOptions
	if message != "" {
		opt = &echotron.CallbackQueryOptions{Text: message, CacheTime: 3600}
	}
	callback.Answer(opt)
	callback.Delete()
}

func buildCalendarMessage(date Date, text string) message.Text {
	var (
		month, monthDays = date.Month()
		weekday          = date.MonthStart().Week()
		buttons          = make([]tgui.InlineButton, monthDays+weekday)
		row              []tgui.InlineButton
		now              = Now().Time()
		msg              = message.Text{text + month.String(), nil}
	)

	for i := 0; i < weekday; i++ {
		buttons[i] = tgui.InlineCaller("🚫", "/alert", "❌ Cannot create an event in this day")
	}

	row = buttons[weekday:]
	date = date.MonthStart()
	for i := range row {
		var (
			label string = strconv.Itoa(i + 1)
			day   Date   = date.Skip(0, 0, i)
		)

		if day.Time().Before(now) {
			row[i] = tgui.InlineCaller("🚫"+label, "/alert", "❌ Cannot create an event in this day")
		} else {
			row[i] = tgui.InlineCaller(label, "/publish", day.Format(), "add")
		}
	}

	keyboard := tgui.Arrange(7, buttons...)
	row = keyboard[len(keyboard)-1]
	for i := len(row); i < 7; i++ {
		row = append(row, tgui.InlineCaller("🚫", "/alert", "❌ Cannot create an event in this day"))
	}
	keyboard[len(keyboard)-1] = row

	return *msg.ClipInlineKeyboard(append(
		[][]tgui.InlineButton{{
			tgui.InlineCaller("⏮", "/publish", date.Skip(0, -1, 0).Format(), "refresh"),
			tgui.InlineCaller(month.String(), "/alert", "This feature is not yet avaiable"),
			tgui.InlineCaller("⏭", "/publish", date.Skip(0, 1, 0).Format(), "refresh"),
		}},
		append(keyboard, []tgui.InlineButton{
			tgui.InlineCaller("❌ Cancel", "/close", "✔️ Operation cancelled"),
			tgui.InlineCaller("🔄Refresh", "/publish", date.Format(), "refresh"),
		})...,
	))
}

func extractCommand(update *message.Update) (command string, payload []string) {
	if update.CallbackQuery != nil {
		command = update.CallbackQuery.Data
	} else if message := update.FromMessage(); message != nil {
		command = message.Text
	} else {
		return
	}

	payload = strings.Split(command, " ")
	return payload[0], payload[1:]
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

func buildDateListMessage(c Calendar, userID int64) message.Text {
	var (
		msg = message.Text{c.name, nil}
		kbd = make([][]tgui.InlineButton, len(c.dates)+1)
		i   = 0
	)

	for date, event := range c.dates {
		var caption string = beautifyDate(date)
		if n := event.countAttendee(); n > 0 {
			if event.hasJoined(userID) {
				caption = "✅ " + caption + " - 👥" + strconv.Itoa(n-1) + " + 1 (You)"
			} else {
				caption += "- 👥" + strconv.Itoa(n)
			}
		}
		kbd[i] = tgui.Wrap(tgui.InlineCaller(caption, "/join", c.invitation, date))
		i++
	}
	kbd[i] = tgui.Wrap(tgui.InlineCaller("❌ Close", "/close"))

	return *msg.ClipInlineKeyboard(kbd)
}
