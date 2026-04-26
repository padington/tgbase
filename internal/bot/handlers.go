package bot

import (
	"fmt"
	"log"
	"os"
	"runtime"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (b *Bot) handleUpdate(update tgbotapi.Update) {
	if update.Message == nil {
		log.Printf("skipping non-message update id=%d", update.UpdateID)
		return
	}
	if !update.Message.IsCommand() {
		b.handleText(update.Message)
		return
	}

	cmd := update.Message.Command()
	log.Printf("command /%s from @%s (chat=%d)", cmd, update.Message.From.UserName, update.Message.Chat.ID)

	switch cmd {
	case "ping":
		b.reply(update.Message, "pong")
	case "whoami":
		b.reply(update.Message, b.whoami())
	case "menu":
		b.sendMenu(update.Message)
	default:
		log.Printf("unknown command: /%s", cmd)
	}
}

func (b *Bot) handleText(msg *tgbotapi.Message) {
	switch msg.Text {
	case "Ping":
		b.reply(msg, "pong")
	case "Whoami":
		b.reply(msg, b.whoami())
	default:
		log.Printf("unhandled text from @%s: %q", msg.From.UserName, msg.Text)
	}
}

func (b *Bot) sendMenu(msg *tgbotapi.Message) {
	keyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Ping"),
			tgbotapi.NewKeyboardButton("Whoami"),
		),
	)
	keyboard.ResizeKeyboard = true
	out := tgbotapi.NewMessage(msg.Chat.ID, "Choose an action:")
	out.ReplyMarkup = keyboard
	if _, err := b.api.Send(out); err != nil {
		log.Printf("send menu to chat=%d: %v", msg.Chat.ID, err)
	}
}

func (b *Bot) whoami() string {
	hostname, _ := os.Hostname()
	env := b.cfg.Env
	if env == "" {
		env = "unknown"
	}
	return fmt.Sprintf(
		"env: %s\nhostname: %s\nos: %s/%s",
		env, hostname, runtime.GOOS, runtime.GOARCH,
	)
}

func (b *Bot) reply(msg *tgbotapi.Message, text string) {
	out := tgbotapi.NewMessage(msg.Chat.ID, text)
	out.ReplyToMessageID = msg.MessageID
	if _, err := b.api.Send(out); err != nil {
		log.Printf("send reply to chat=%d: %v", msg.Chat.ID, err)
		return
	}
	log.Printf("replied to @%s (chat=%d): %q", msg.From.UserName, msg.Chat.ID, text)
}
