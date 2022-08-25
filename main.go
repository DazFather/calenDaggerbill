package main

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/DazFather/parrbot/message"
	"github.com/DazFather/parrbot/robot"
	"github.com/DazFather/parrbot/tgui"

	"github.com/NicoNex/echotron/v3"
)

func beautifyDate(date string) string {
	return strings.Replace(date, "T", " ", 1)
}

var (
	organizers      = map[int64]*Calendar{}
	DEFAULT_MSG_OPT = &echotron.MessageOptions{ParseMode: "HTML"}
)

func main() {
	go clearUnused(time.Hour * 24 * 30 * 6)
	robot.Start(startHandler, joinHandler, publishHandler, closeHandler, alertHandler, editHandler, setHandler)
}

func clearUnused(every time.Duration) {
	c := time.Tick(every)
	for _ = range c {
		for userID, calendar := range organizers {
			if len(calendar.dates) == 0 && calendar.UnusedFor() >= every {
				delete(organizers, userID)
			}
		}
	}
}

var startHandler = robot.Command{
	Description: "Start the bot",
	Trigger:     "/start",
	ReplyAt:     message.MESSAGE + message.CALLBACK_QUERY,
	CallFunc: func(bot *robot.Bot, update *message.Update) message.Any {
		var _, payload = extractCommand(update)
		if len(payload) == 0 {
			var (
				now  string = Now().Format()
				text string
				opts = tgui.ParseModeOpt(nil, DEFAULT_MSG_OPT.ParseMode)
			)

			if calendar := organizers[bot.ChatID]; calendar != nil {
				now := Now().Format()
				text = "🐦 <i>Hi! What can I do for you today?</i>\n" +
					"Here is some infos about your calendar:" +
					"\n🔔notification: <code>" + toString(calendar.notification) + "</code>" +
					"\n🎟incoming events: " + strconv.Itoa(len(calendar.dates)) +
					"\n👥people reached: " + strconv.Itoa(len(calendar.AllCurrentAttendee())) +
					"\n🏷name: <code>" + calendar.name + "</code>" +
					"\n📑description: <code>" + calendar.description + "</code>"

				tgui.InlineKbdOpt(opts, [][]tgui.InlineButton{
					{tgui.InlineCaller("➕ Add events", "/publish", now)},
					{tgui.InlineCaller("📝 Edit calendar", "/edit")},
				})
			} else {
				text = "👋 <b>Welcome, I'm Calen-Daggerbill!</b> 🐦\n" +
					"<i>Your <a href=\"https://github.com/DazFather/calendaggerbill\">open source</a>" +
					" robo-hummingbird that will assist you to mange your calendar</i>" +
					"\n\nUsing me is very easy and free:" +
					"\n First of all you need to create a calendar, " +
					"<i>use the button below or the command or the command</i> /publish"

				tgui.InlineKbdOpt(opts, [][]tgui.InlineButton{{
					tgui.InlineCaller("🆕 Create new calendar", "/publish", now),
				}})
				tgui.DisableWebPagePreview(opts)
			}
			tgui.ShowMessage(*update, text, opts)
			return nil
		}

		if calendar := retreiveCalendar(payload[0]); calendar != nil {
			return buildDateListMessage(*calendar, bot.ChatID)
		}
		return buildErrorMessage("Invalid invitation link")
	},
}

var joinHandler = robot.Command{
	Trigger: "/join",
	ReplyAt: message.CALLBACK_QUERY,
	CallFunc: func(bot *robot.Bot, update *message.Update) message.Any {
		var calendar *Calendar

		if _, payload := extractCommand(update); len(payload) != 2 {
			return buildErrorMessage("Invalid joining: " + update.CallbackQuery.Data)
		} else if c, err := JoinEvent(*update.CallbackQuery.From, payload[0], payload[1]); err != nil {
			return buildErrorMessage(err.Error())
		} else {
			calendar = c
		}

		collapse(update.CallbackQuery, "✅ You joined this event")
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
			if callback := update.CallbackQuery; callback != nil {
				msg = buildErrorMessage("No given payload")
			} else {
				msg = buildCalendarMessage(Now(), "🗓 Select a day from the calendar: ")
			}
		case 1:
			var selected Date
			if d := ParseDate(payload[0]); d == nil {
				msg = buildErrorMessage("Invaid date: " + payload[0])
				break
			} else {
				selected = *d
			}

			msg = buildCalendarMessage(selected, "🗓 Select a day from the calendar: ")
		case 2:
			var date Date
			if d := ParseDate(payload[0]); d == nil {
				msg = buildErrorMessage("Invaid date: " + payload[0])
				break
			} else {
				date = *d
			}

			if payload[1] == "refresh" {
				msg = buildCalendarMessage(date, "🗓 Select a day from the calendar: ")
				break
			}

			var (
				hasCalendar bool = organizers[bot.ChatID] != nil
				link             = GetShareLink(*AddToCalendar(*update.CallbackQuery.From, date))
			)
			notify(update.CallbackQuery, "✔️ Date: 📅"+beautifyDate(date.Format())+" added to your calendar ")
			if hasCalendar {
				break
			}
			opt := *DEFAULT_MSG_OPT
			m := message.Text{
				"✅ <b>You calendar has been created</b>\n" +
					"Use the previous message or /publish again to add a new avaiable dates\n" +
					"Send /edit to modify your calendar's settings like name, description and notification\n" +
					"Share the following link to make people join your events: " + link,
				&opt,
			}
			return *m.ClipInlineKeyboard([][]tgui.InlineButton{{
				tgui.InlineCaller("🔙 Back", "/start"),
				tgui.InlineCaller("❎ Close", "/close"),
			}})
		}

		if callback := update.CallbackQuery; callback != nil {
			callback.Delete()
		} else {
			update.Message.Delete()
		}

		return msg
	},
}

var editHandler = robot.Command{
	Description: "Edit your calendar",
	Trigger:     "/edit",
	ReplyAt:     message.MESSAGE + message.CALLBACK_QUERY,
	CallFunc: func(bot *robot.Bot, update *message.Update) message.Any {
		var (
			calendar                  = organizers[bot.ChatID]
			current, field, suggested string
		)
		if update.Message != nil {
			update.Message.Delete()
		}
		if calendar == nil {
			return buildErrorMessage("You don't have a calendar yet, use the command /publish to create a new one")
		}

		if _, payload := extractCommand(update); len(payload) < 2 {
			options := *DEFAULT_MSG_OPT
			options.ReplyMarkup = tgui.InlineKeyboard([][]tgui.InlineButton{{
				tgui.InlineCaller("❌ Cancel", "/close", "✔️ Operation cancelled"),
			}})
			return message.Text{
				"🆘 Use this command to edit your calendar, at the moment you can change name, description and notification\n" +
					"To do so just use the command followed by what you want to edit " +
					"(<code>name</code>, <code>description</code> or <code>notification</code>)" +
					" and then the new value, ex:\n <code>/edit name My new AMAZING✨ name</code>" +
					"\nFor notification the allowed values are <code>on</code> or <code>off</code> only",
				&options,
			}
		} else {
			field, suggested = strings.ToLower(payload[0]), strings.Join(payload[1:], " ")
		}

		switch field {
		case "name":
			current = calendar.name
		case "description":
			current = calendar.description
		case "notification":
			if toBool(suggested) == nil {
				return buildErrorMessage("Invaild specifier for this command (" + suggested + "), use <code>on</code>, <code>off</code> instead")
			}
			current = toString(calendar.notification)
		default:
			return buildErrorMessage("Invaild specifier for this command: \"<i>" + field + "</i>\", use <code>name</code> or <code>description</code> instead")
		}

		tgui.ShowMessage(*update,
			"📝 Your calendar's "+field+" will be change\n"+
				"<i>from:</i> <code>"+current+"</code>\n<i>to:</i> <code>"+suggested+"</code>\n"+
				"\n<b>Confirm the change?</b>",
			tgui.InlineKbdOpt(tgui.ParseModeOpt(nil, DEFAULT_MSG_OPT.ParseMode),
				[][]tgui.InlineButton{{
					tgui.InlineCaller("✅ Confirm", "/set", field, suggested),
					tgui.InlineCaller("❌ Cancel", "/close", "✔️ Operation cancelled"),
				}},
			),
		)
		return nil
	},
}

var setHandler = robot.Command{
	Trigger: "/set",
	ReplyAt: message.CALLBACK_QUERY,
	CallFunc: func(bot *robot.Bot, update *message.Update) message.Any {
		var (
			callback               *message.CallbackQuery = update.CallbackQuery
			field, value, previous string
			calendar               *Calendar = organizers[bot.ChatID]
			needWarning            bool
		)
		if calendar == nil {
			collapse(callback, "🚫 Unable to set: no calendar found")
			return nil
		}
		if _, payload := extractCommand(update); len(payload) < 2 {
			collapse(callback, "🚫 Unable to set: invalid command")
			return nil
		} else {
			field, value = payload[0], strings.Join(payload[1:], " ")
		}

		switch field {
		case "notification":
			previous = toString(calendar.notification)
			calendar.notification = *toBool(value)
		case "name":
			previous = calendar.name
			calendar.name = value
			needWarning = true
		case "description":
			previous = calendar.description
			calendar.description = value
			needWarning = true
		default:
			collapse(callback, "🚫 Unable to set: invalid field")
		}

		text := "✅ <b>Your calendar has been edited</b>\nCalendar's " + field + " successfully changed to:\n " + value
		if needWarning {
			attendee := calendar.AllCurrentAttendee()
			for _, userID := range attendee {
				message.Text{
					"❕ The " + field + " of a calendar that you have joined changed:\n<i>" + previous + "</i> ➡️ <b>" + value + "</b>",
					DEFAULT_MSG_OPT,
				}.Send(int64(userID))
			}
			if tot := len(attendee); tot > 0 {
				text += ", all " + strconv.Itoa(tot) + " attendee has been warned"
			}
		}

		tgui.ShowMessage(*update, text, tgui.InlineKbdOpt(
			tgui.ParseModeOpt(nil, "HTML"),
			[][]tgui.InlineButton{{
				tgui.InlineCaller("↩️ Turn "+field+" back to "+previous, "/edit", field, previous),
				tgui.InlineCaller("❎ Close", "/close"),
			}},
		))
		return nil
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

func toBool(on_off string) (value *bool) {
	switch strings.ToLower(on_off) {
	case "on":
		value = new(bool)
		*value = true
	case "off":
		value = new(bool)
		*value = false
	}
	return
}

func toString(value bool) string {
	if value {
		return "on"
	}
	return "off"
}

func AddToCalendar(user echotron.User, dates ...Date) *Calendar {
	var calendar *Calendar = organizers[user.ID]
	if calendar == nil {
		calendar = NewCalendar(
			user.FirstName+" calendar",
			user.FirstName+" personal event",
			strconv.Itoa(int(user.ID)),
		)
		calendar.dates = make(map[string]*Event, len(dates))
		organizers[user.ID] = calendar
	}

	for _, date := range dates {
		if !calendar.addDate(date) {
			continue
		}

		var strDate string = beautifyDate(date.Format())

		date.Skip(0, 0, -7).When(
			Remind(calendar, date, "🔔 Don't forget the "+calendar.name+", is cooming soon: "+strDate),
		)

		date.Skip(0, 0, -1).When(
			Remind(calendar, date, "🔔 Tomorrow "+strDate+", there will be "+calendar.name+" waiting for you!"),
		)

		date.When(func() {
			calendar.removeDate(date)
		})
	}

	return calendar
}

func JoinEvent(user echotron.User, invitation, rawDate string) (calendar *Calendar, err error) {
	var ownerID *int64 = retreiveOwner(invitation)
	if ownerID == nil {
		return nil, errors.New("Invalid invitation")
	}

	if date := ParseDate(rawDate); date == nil {
		err = errors.New("Invalid date")
	} else {
		calendar = organizers[*ownerID]
		err = calendar.joinDate(*date, user.ID)
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
		if tot := calendar.dates[rawDate].countAttendee() - 1; tot > 0 {
			count = string(tot) + " " + count
		}
		sendNotification(*ownerID, "<b>"+count+"</b>: "+name+" joined your event in date: "+beautifyDate(rawDate))
	}

	return
}

func Remind(calendar *Calendar, date Date, text string) (reminder func()) {
	return func() {
		if calendar == nil || !calendar.notification {
			return
		}

		for userID := range calendar.CurrentAttendee(date) {
			message.Text{text, nil}.Send(int64(userID))
		}
	}
}

func GetShareLink(c Calendar) string {
	var res, err = message.API().GetMe()
	if err != nil || res.Result == nil {
		return "/start " + c.invitation
	}
	return "t.me/" + res.Result.Username + "?start=" + c.invitation
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

func buildCalendarMessage(date Date, text string) message.Text {
	var (
		month, monthDays = date.Month()
		weekday          = date.MonthStart().Week()
		buttons          = make([]tgui.InlineButton, monthDays+weekday)
		row              []tgui.InlineButton
		now              = Now()
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

		if day.IsBefore(now) {
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

func buildDateListMessage(c Calendar, userID int64) message.Text {
	var (
		msg = message.Text{
			"🛎 <b>" + c.name + "</b>\n" + c.description + "\n\n<i>Tap one (or more) of following dates to join</i>",
			DEFAULT_MSG_OPT,
		}
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
	kbd[i] = tgui.Wrap(tgui.InlineCaller("❎ Close", "/close"))

	return *msg.ClipInlineKeyboard(kbd)
}

func buildErrorMessage(text string) message.Text {
	var opt = *DEFAULT_MSG_OPT
	opt.ReplyMarkup = tgui.InlineKeyboard([][]tgui.InlineButton{{
		tgui.InlineCaller("❎ Close", "/close"),
	}})
	return message.Text{"🚫 " + text, &opt}
}

func sendNotification(chatID int64, text string) error {
	var msg = message.Text{"🔔" + text, DEFAULT_MSG_OPT}

	_, err := msg.ClipInlineKeyboard([][]tgui.InlineButton{{
		tgui.InlineCaller("❎ Close", "/close", "✔️ Notification deleted"),
		tgui.InlineCaller("🔕 Turn off notifications", "/edit", "notification", "off"),
	}}).Send(chatID)

	return err
}
