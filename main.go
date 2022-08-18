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

type Event struct {
	name  string
	dates map[string][]int64
}

func (e *Event) join(date string, userID int64) {
	e.dates[date] = append(e.dates[date], userID)
}

func (e Event) hasJoined(date string, userID int64) bool {
	for _, guestID := range e.dates[date] {
		if guestID == userID {
			return true
		}
	}
	return false
}

var organizers = map[int64]*Event{
	169090723: &Event{
		name: "MEGA EVENTO DELLA VITA",
		dates: map[string][]int64{
			"poi_vediamo": nil,
		},
	},
}

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

		if event := retreiveEvent(payload[0]); event != nil {
			return buildDateListMessage(*event, payload[0], bot.ChatID)
		}
		return message.Text{"Invalid invitation link", nil}
	},
}

var joinHandler = robot.Command{
	Trigger: "/join",
	ReplyAt: message.CALLBACK_QUERY,
	CallFunc: func(bot *robot.Bot, update *message.Update) message.Any {
		var _, payload = extractCommand(update)
		if len(payload) != 2 {
			return message.Text{"Invalid joining: " + update.CallbackQuery.Data, nil}
		}

		if event := retreiveEvent(payload[0]); event != nil {
			event.join(payload[1], update.CallbackQuery.From.ID)
			update.CallbackQuery.Answer(&echotron.CallbackQueryOptions{
				Text:      "✅ You joined this event",
				CacheTime: 3600,
			})
			update.CallbackQuery.Delete()
			return buildDateListMessage(*event, payload[0], bot.ChatID)
		}
		return message.Text{"Invalid invitation link", nil}
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

		pusblishEvent(bot.ChatID, Event{
			name:  update.CallbackQuery.From.FirstName + " personal event",
			dates: map[string][]int64{payload[0]: nil},
		})
		update.CallbackQuery.Delete()

		return message.Text{
			"Event published share this command to make people join:\n/start " + strconv.Itoa(int(bot.ChatID)) + "\n",
			nil,
		}
	},
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

func pusblishEvent(userID int64, event Event) {
	organizers[userID] = &event
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

func retreiveEvent(invitation string) *Event {
	userID, err := strconv.Atoi(invitation)
	if err != nil {
		return nil
	}

	return organizers[int64(userID)]
}

func buildDateListMessage(event Event, invitation string, userID int64) message.Text {
	var (
		msg = message.Text{event.name, nil}
		kbd = make([][]tgui.InlineButton, len(event.dates))
		i   = 0
	)

	for date, joiner := range event.dates {
		var caption string = date
		if n := len(joiner); n > 0 {
			if event.hasJoined(date, userID) {
				caption = "✅ " + caption + " - 👥" + strconv.Itoa(n-1) + " + 1 (You)"
			} else {
				caption += "- 👥" + strconv.Itoa(n)
			}
		}
		kbd[i] = tgui.Wrap(tgui.InlineCaller(caption, "/join", invitation, date))
		i++
	}

	return *msg.ClipInlineKeyboard(kbd)
}
