package main

import (
	//"fmt"
	"strconv"
	"strings"

	"github.com/DazFather/parrbot/message"
	"github.com/DazFather/parrbot/robot"
	"github.com/DazFather/parrbot/tgui"

	"github.com/NicoNex/echotron/v3"
)

var organizers = map[int64]*Calendar{}

func main() {
	robot.Start(startHandler, joinHandler, publishHandler)
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
		} else {
			date, ownerID, calendar = payload[1], *userID, organizers[*userID]
		}

		if err := calendar.joinDate(date, bot.ChatID); err != nil {
			return message.Text{"🚫 " + err.Error(), nil}
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
		var _, payload = extractCommand(update)
		if len(payload) == 0 {
			return buildCalendarMessage("Select a day")
		}

		if calendar := organizers[bot.ChatID]; calendar != nil {
			calendar.addDate(payload[0])
			collapse(update.CallbackQuery, "📅 Date: "+payload[0]+" added to your calendar ✔️")
			return nil
		}

		calendar := AddCalendar(*update.CallbackQuery.From, payload[0])

		tgui.ShowMessage(
			*update,
			strings.Join([]string{
				"✔️ <i>You calendar has been created</i>",
				"Use again command /publish to add a new avaiable date",
				"Share this to make people join: <code>/start " + calendar.invitation + "</code>",
			}, "\n"),
			tgui.ParseModeOpt(nil, "HTML"),
		)
		return nil
	},
}

func AddCalendar(user echotron.User, dates ...string) *Calendar {
	var calendar = &Calendar{
		name:        user.FirstName + " calendar",
		description: user.FirstName + " personal event",
		invitation:  strconv.Itoa(int(user.ID)),
		dates:       make(map[string]*Event),
	}
	for _, date := range dates {
		calendar.addDate(date)
	}

	organizers[user.ID] = calendar
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

func buildCalendarMessage(text string) message.Text {
	var msg = message.Text{"Invalid invitation link", nil}

	buttons := make([]tgui.InlineButton, 30)
	for i := range buttons {
		day := strconv.Itoa(i + 1)
		buttons[i] = tgui.InlineCaller(day, "/publish", day)
	}

	msg.ClipInlineKeyboard(tgui.Arrange(7, buttons...))
	return msg
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
		kbd = make([][]tgui.InlineButton, len(c.dates))
		i   = 0
	)

	for date, event := range c.dates {
		var caption string = date
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

	return *msg.ClipInlineKeyboard(kbd)
}
