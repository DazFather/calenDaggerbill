package main

import (
	"fmt"
	"strings"

	"github.com/DazFather/parrbot/message"
	"github.com/DazFather/parrbot/robot"
	"github.com/DazFather/parrbot/tgui"
)

func main() {
	// Start cleaning unused calendars job
	go Repeat(DEFAULT_UNUSED_TIME, UnusedCalendarsRemover(DEFAULT_UNUSED_TIME))
	// Start the bot with the following commands:
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

/* --- BOT COMMAND --- */

var startHandler = robot.Command{
	Description: "Main menu",
	Trigger:     "/start",
	ReplyAt:     message.MESSAGE + message.CALLBACK_QUERY,
	CallFunc: func(bot *robot.Bot, update *message.Update) message.Any {
		var payload = extractPayload(update)
		if len(payload) == 0 {
			var (
				now  string = string(Today())
				text string
				opts = genDefaultEditOpt()
			)

			if calendar := CalendarOf(bot.ChatID); calendar != nil {
				text = fmt.Sprint(LOGO, " <i>Hi! What can I do for you today?</i>\n",
					"Here is some infos about your calendar:",
					"\n", NOTIF_ON, "notification: <code>", calendar.notification, "</code>",
					"\n🎟incoming events: ", len(calendar.dates),
					"\n", PEOPLE, "people reached: ", len(calendar.AllCurrentAttendee()),
					"\n🏷name: <code>", calendar.name, "</code>",
					"\n📑description: <code>", calendar.description, "</code>",
				)

				tgui.InlineKbdOpt(opts, [][]tgui.InlineButton{
					{tgui.InlineCaller("➕ Add events", "/publish", now)},
					{tgui.InlineCaller("📝 Edit calendar", "/edit")},
					{tgui.InlineCaller("📨 Invite users", "/link")},
				})
			} else {
				text = fmt.Sprint("👋 <b>Welcome, I'm Calen-Daggerbill!</b> ", LOGO, "\n",
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

		if payload := extractPayload(update); len(payload) != 2 {
			return buildErrorMessage("Invalid joining: " + update.CallbackQuery.Data)
		} else if c, err := JoinEvent(*update.CallbackQuery.From, payload[0], payload[1]); err != nil {
			return buildErrorMessage(err.Error())
		} else {
			calendar = c
		}

		Collapse(update.CallbackQuery, DONE, "You joined this event")
		return buildDateListMessage(*calendar, bot.ChatID)
	},
}

var publishHandler = robot.Command{
	Description: "Publish a new event",
	Trigger:     "/publish",
	ReplyAt:     message.MESSAGE + message.CALLBACK_QUERY,
	CallFunc: func(bot *robot.Bot, update *message.Update) message.Any {
		var msg message.Any

		switch payload := extractPayload(update); len(payload) {
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
				hasCalendar bool = CalendarOf(bot.ChatID) != nil
				link             = GetShareLink(botUsername(), *AddToCalendar(*update.CallbackQuery.From, date))
			)
			Notify(update.CallbackQuery, DONE, fmt.Sprint("Date: ", CALENDAR, " ", date, " added to your calendar "))
			if hasCalendar {
				break
			}
			return genDefaultMessage(
				DONE,
				fmt.Sprint(
					"<b>You calendar has been created</b>\n",
					"Use the previous message or /publish again to add a new avaiable dates\n",
					"Send /edit to modify your calendar's settings like name, description and notification\n",
					"Share the following link to make people join your events: ", link,
				),
				[]tgui.InlineButton{
					tgui.InlineCaller("🔙 Back", "/start"),
					BTN_CLOSE,
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
			calendar         = CalendarOf(bot.ChatID)
			current          string
			field, suggested string = extractFieldValue(update)
		)
		if update.Message != nil {
			update.Message.Delete()
		}
		if calendar == nil {
			return buildErrorMessage("You don't have a calendar yet, use the command /publish to create a new one")
		}

		if field == "" || suggested == "" {
			return genDefaultMessage(
				icon("🆘"),
				fmt.Sprint(
					"Use this command to edit your calendar, at the moment you can change name, description and notification\n",
					"To do so just use the command followed by what you want to edit ",
					"(<code>name</code>, <code>description</code> or <code>notification</code>)",
					" and then the new value, ex:\n <code>/edit name My new AMAZING✨ name</code>",
					"\nFor notification the allowed values are <code>on</code> or <code>off</code> only",
				),
				tgui.Wrap(BTN_CANCEL),
			)
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
			fmt.Sprint("📝 Your calendar's ", field, " will change\n",
				"<i>from:</i> <code>", current, "</code>\n",
				"<i>to:</i> <code>", suggested, "</code>\n",
				"\n<b>Confirm the change?</b>",
			),
			genDefaultEditOpt([]tgui.InlineButton{
				tgui.InlineCaller(CONFIRM.Text("Confirm"), "/set", field, suggested),
				BTN_CANCEL,
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
			callback     *message.CallbackQuery = update.CallbackQuery
			field, value string                 = extractFieldValue(update)
			previous     string
			calendar     *Calendar = CalendarOf(bot.ChatID)
			needWarning  bool
		)
		if calendar == nil {
			Collapse(callback, BLOCK, "Unable to set: no calendar found")
			return nil
		}
		if field == "" && value == "" {
			Collapse(callback, BLOCK, "Unable to set: invalid command")
			return nil
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
			Collapse(callback, BLOCK, "Unable to set: invalid field")
		}

		text := "<b>Your calendar has been edited</b>\nCalendar's " + field + " successfully changed to:\n " + value
		if needWarning {
			attendee := calendar.AllCurrentAttendee()
			for _, userID := range attendee {
				genDefaultMessage(icon("❕"), fmt.Sprint(
					"The ", field, " of a calendar that you have joined changed:\n<i>", previous, "</i> ➡️ <b>", value, "</b>",
				)).Send(int64(userID))
			}
			if tot := len(attendee); tot > 0 {
				text += fmt.Sprint(", all ", tot, " attendee has been warned")
			}
		}

		tgui.ShowMessage(*update, DONE.Text(text), genDefaultEditOpt([]tgui.InlineButton{
			tgui.InlineCaller("↩️ Turn "+field+" back to "+previous, "/edit", field, previous),
			BTN_CLOSE,
		}))
		return nil
	},
}

var linkHandler = robot.Command{
	Description: "Get the shareable link to your calendar",
	Trigger:     "/link",
	ReplyAt:     message.MESSAGE + message.CALLBACK_QUERY,
	CallFunc: func(bot *robot.Bot, update *message.Update) message.Any {
		var calendar = CalendarOf(bot.ChatID)
		if calendar == nil {
			err := "You don't have a calendar yet, use the command /publish to create a new one"
			if update.CallbackQuery == nil {
				update.Message.Delete()
				return buildErrorMessage(err)
			}
			Notify(update.CallbackQuery, BLOCK, err)
		}

		tgui.ShowMessage(*update, "Your link: "+GetShareLink(botUsername(), *calendar), genDefaultEditOpt([]tgui.InlineButton{
			tgui.InlineCaller("🔙 Back", "/start"),
			BTN_CLOSE,
		}))
		return nil
	},
}

/* --- UTILITIES --- */

// extractText grabs the text from a given update
func extractText(update *message.Update) (content string) {
	if update.CallbackQuery != nil {
		content = update.CallbackQuery.Data
	} else if msg := update.FromMessage(); msg != nil {
		content = msg.Text
	}

	return
}

// extractPayload extract the command from an update, remove trigger and split the rest
func extractPayload(update *message.Update) (payload []string) {
	var command string = extractText(update)
	if command == "" {
		return
	}

	if values := strings.Split(command, " "); len(values) > 1 {
		payload = values[1:]
	}

	return
}

// extractFieldValue extract the command from an update, remove trigger and split
// only the next word to the rest (used in /set and /edit)
func extractFieldValue(update *message.Update) (field string, value string) {
	var command = extractText(update)
	if command == "" {
		return
	}

	if ind := strings.IndexRune(command, ' '); ind > 0 {
		command = strings.TrimSpace(command[ind+1:])
		ind = strings.IndexRune(command, ' ')
		if ind > 0 {
			field, value = command[:ind], command[ind+1:]
		}
	}
	return
}

func botUsername() (username string) {
	var res, err = message.API().GetMe()
	if err == nil && res.Result != nil {
		username = res.Result.Username
	}
	return
}
