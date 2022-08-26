package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/DazFather/parrbot/message"
	"github.com/DazFather/parrbot/robot"
	"github.com/DazFather/parrbot/tgui"

	"github.com/NicoNex/echotron/v3"
)

var organizers = map[int64]*Calendar{}

func main() {
	clearUnused()
	robot.Start(
		startHandler,   // start menu & handle join link
		joinHandler,    // confirm join
		publishHandler, // create a new calendar
		closeHandler,   // close any menu and show toast alert
		alertHandler,   // show toast alert
		editHandler,    // edit calendar menu
		setHandler,     // confirm edit calendar menu
		linkHandler,    // show shareable link
	)
}

func clearUnused() {
	go Repeat(UNUSED_TIME, func() {
		for userID, calendar := range organizers {
			if calendar.IsUnused() {
				delete(organizers, userID)
			}
		}
	})
}

var startHandler = robot.Command{
	Description: "Main menu",
	Trigger:     "/start",
	ReplyAt:     message.MESSAGE + message.CALLBACK_QUERY,
	CallFunc: func(bot *robot.Bot, update *message.Update) message.Any {
		var _, payload = extractCommand(update)
		if len(payload) == 0 {
			var (
				now  string = string(Today())
				text string
				opts = genDefaultEditOpt()
			)

			if calendar := organizers[bot.ChatID]; calendar != nil {
				text = fmt.Sprint("🐦 <i>Hi! What can I do for you today?</i>\n",
					"Here is some infos about your calendar:",
					"\n🔔notification: <code>", calendar.notification, "</code>",
					"\n🎟incoming events: ", len(calendar.dates),
					"\n👥people reached: ", len(calendar.AllCurrentAttendee()),
					"\n🏷name: <code>", calendar.name, "</code>",
					"\n📑description: <code>", calendar.description, "</code>",
				)

				tgui.InlineKbdOpt(opts, [][]tgui.InlineButton{
					{tgui.InlineCaller("➕ Add events", "/publish", now)},
					{tgui.InlineCaller("📝 Edit calendar", "/edit")},
					{tgui.InlineCaller("📨 Invite users", "/link")},
				})
			} else {
				text = fmt.Sprint("👋 <b>Welcome, I'm Calen-Daggerbill!</b> 🐦\n",
					"<i>Your <a href=\"https://github.com/DazFather/calendaggerbill\">open source</a>",
					" robo-hummingbird that will assist you to mange your calendar</i>",
					"\n\nUsing me is very easy and free:",
					"\n First of all you need to create a calendar, ",
					"<i>use the button below or the command </i> /publish",
				)

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
			var date, err = ParseDate(payload[0])
			if err != nil {
				msg = buildErrorMessage("Invaid date: " + err.Error())
				break
			}

			msg = buildCalendarMessage(date, "🗓 Select a day from the calendar: ")
		case 2:
			var date, err = ParseDate(payload[0])
			if err != nil {
				msg = buildErrorMessage("Invaid date: " + err.Error())
				break
			}

			if payload[1] == "refresh" {
				msg = buildCalendarMessage(date, "🗓 Select a day from the calendar: ")
				break
			}

			var (
				hasCalendar bool = organizers[bot.ChatID] != nil
				link             = GetShareLink(*AddToCalendar(*update.CallbackQuery.From, date))
			)
			notify(update.CallbackQuery, fmt.Sprint("✔️ Date: 📅", date, " added to your calendar "))
			if hasCalendar {
				break
			}
			return genDefaultMessage(
				fmt.Sprint(
					"✅ <b>You calendar has been created</b>\n",
					"Use the previous message or /publish again to add a new avaiable dates\n",
					"Send /edit to modify your calendar's settings like name, description and notification\n",
					"Share the following link to make people join your events: ", link,
				),
				[]tgui.InlineButton{
					tgui.InlineCaller("🔙 Back", "/start"),
					tgui.InlineCaller("❎ Close", "/close"),
				},
			)
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
			return genDefaultMessage(
				fmt.Sprint(
					"🆘 Use this command to edit your calendar, at the moment you can change name, description and notification\n",
					"To do so just use the command followed by what you want to edit ",
					"(<code>name</code>, <code>description</code> or <code>notification</code>)",
					" and then the new value, ex:\n <code>/edit name My new AMAZING✨ name</code>",
					"\nFor notification the allowed values are <code>on</code> or <code>off</code> only",
				),
				tgui.Wrap(tgui.InlineCaller("❌ Cancel", "/close", "✔️ Operation cancelled")),
			)
		} else {
			field, suggested = strings.ToLower(payload[0]), strings.Join(payload[1:], " ")
		}

		switch field {
		case "name":
			current = calendar.name
		case "description":
			current = calendar.description
		case "notification":
			if suggested == "toggle" {
				suggested = calendar.notification.Toggle().String()
			} else if ParseToggler(suggested) == nil {
				return buildErrorMessage("Invaild specifier for this command (" + suggested + "), use <code>on</code>, <code>off</code> instead")
			}
			current = calendar.notification.String()
		default:
			return buildErrorMessage("Invaild specifier for this command: \"<i>" + field + "</i>\", use <code>name</code> or <code>description</code> instead")
		}

		tgui.ShowMessage(*update,
			fmt.Sprint("📝 Your calendar's ", field, " will be change\n",
				"<i>from:</i> <code>", current, "</code>\n",
				"<i>to:</i> <code>", suggested, "</code>\n",
				"\n<b>Confirm the change?</b>",
			),
			genDefaultEditOpt([]tgui.InlineButton{
				tgui.InlineCaller("✅ Confirm", "/set", field, suggested),
				tgui.InlineCaller("❌ Cancel", "/close", "✔️ Operation cancelled"),
			}),
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
			previous = calendar.notification.String()
			calendar.notification = *ParseToggler(value)
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
				genDefaultMessage(fmt.Sprint(
					"❕ The ", field, " of a calendar that you have joined changed:\n<i>", previous, "</i> ➡️ <b>", value, "</b>",
				)).Send(int64(userID))
			}
			if tot := len(attendee); tot > 0 {
				text += fmt.Sprint(", all ", tot, " attendee has been warned")
			}
		}

		tgui.ShowMessage(*update, text, genDefaultEditOpt([]tgui.InlineButton{
			tgui.InlineCaller("↩️ Turn "+field+" back to "+previous, "/edit", field, previous),
			tgui.InlineCaller("❎ Close", "/close"),
		}))
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

var linkHandler = robot.Command{
	Description: "Get the shareable link to your calendar",
	Trigger:     "/link",
	ReplyAt:     message.MESSAGE + message.CALLBACK_QUERY,
	CallFunc: func(bot *robot.Bot, update *message.Update) message.Any {
		var calendar = organizers[bot.ChatID]
		if calendar == nil {
			err := "You don't have a calendar yet, use the command /publish to create a new one"
			if update.CallbackQuery == nil {
				update.Message.Delete()
				return buildErrorMessage(err)
			}
			notify(update.CallbackQuery, "🚫 "+err)
		}

		tgui.ShowMessage(*update, "Your link: "+calendar.invitation, genDefaultEditOpt([]tgui.InlineButton{
			tgui.InlineCaller("🔙 Back", "/start"),
			tgui.InlineCaller("❎ Close", "/close"),
		}))
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
			Remind(calendar, timestamp, fmt.Sprint("🔔 Don't forget the ", calendar.name, ", is cooming soon: ", date)),
		)

		date.Skip(0, 0, -1).WhenOccurrs(
			Remind(calendar, timestamp, fmt.Sprint("🔔 Tomorrow ", date, ", there will be ", calendar.name, " waiting for you!")),
		)

		date.WhenOccurrs(func() {
			calendar.removeDate(timestamp)
		})
	}

	return calendar
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

func Remind(calendar *Calendar, date FormattedDate, text string) (reminder func()) {
	return func() {
		if calendar == nil || !calendar.notification {
			return
		}

		for userID := range calendar.CurrentAttendee(date) {
			genDefaultMessage(text).Send(int64(userID))
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
	} else if msg := update.FromMessage(); msg != nil {
		command = msg.Text
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
