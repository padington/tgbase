package meta

import (
	"fmt"
	"os"
	"runtime"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/padington/tgbase/internal/router"
)

func Ping() router.HandlerFunc {
	return func(sender router.Sender, msg *tgbotapi.Message) {
		reply(sender, msg, "pong")
	}
}

func Whoami(env string) router.HandlerFunc {
	return func(sender router.Sender, msg *tgbotapi.Message) {
		hostname, _ := os.Hostname()
		if env == "" {
			env = "unknown"
		}
		text := fmt.Sprintf("env: %s\nhostname: %s\nos: %s/%s", env, hostname, runtime.GOOS, runtime.GOARCH)
		reply(sender, msg, text)
	}
}

func Menu() router.HandlerFunc {
	return func(sender router.Sender, msg *tgbotapi.Message) {
		keyboard := tgbotapi.NewReplyKeyboard(
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("Ping"),
				tgbotapi.NewKeyboardButton("Whoami"),
			),
		)
		keyboard.ResizeKeyboard = true
		out := tgbotapi.NewMessage(msg.Chat.ID, "Choose an action:")
		out.ReplyMarkup = keyboard
		sender.Send(out)
	}
}

func reply(sender router.Sender, msg *tgbotapi.Message, text string) {
	out := tgbotapi.NewMessage(msg.Chat.ID, text)
	out.ReplyToMessageID = msg.MessageID
	sender.Send(out)
}
