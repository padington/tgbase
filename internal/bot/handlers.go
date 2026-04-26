package bot

import (
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (b *Bot) handleUpdate(update tgbotapi.Update) {
	if update.Message == nil || !update.Message.IsCommand() {
		return
	}

	switch update.Message.Command() {
	case "ping":
		b.reply(update.Message, "pong")
	}
}

func (b *Bot) reply(msg *tgbotapi.Message, text string) {
	out := tgbotapi.NewMessage(msg.Chat.ID, text)
	out.ReplyToMessageID = msg.MessageID
	if _, err := b.api.Send(out); err != nil {
		log.Printf("send reply: %v", err)
	}
}
