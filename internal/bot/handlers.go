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
		log.Printf("message from @%s (chat=%d): %q — not a command, ignoring",
			update.Message.From.UserName, update.Message.Chat.ID, update.Message.Text)
		return
	}

	cmd := update.Message.Command()
	log.Printf("command /%s from @%s (chat=%d)", cmd, update.Message.From.UserName, update.Message.Chat.ID)

	switch cmd {
	case "ping":
		b.reply(update.Message, "pong")
	case "whoami":
		b.reply(update.Message, b.whoami())
	default:
		log.Printf("unknown command: /%s", cmd)
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
