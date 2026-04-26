package survey

import (
	"fmt"
	"log"
	"strconv"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/padington/tgbase/internal/router"
	"github.com/padington/tgbase/internal/state"
)

// Start handles /start — sends the "How are you?" question and moves user to awaiting state.
func Start(store *state.Store) router.HandlerFunc {
	return func(sender router.Sender, msg *tgbotapi.Message) {
		d := store.Get(msg.From.ID)
		d.State = state.StateAwaitingHowamiAnswer
		store.Set(msg.From.ID, d)

		keyboard := tgbotapi.NewReplyKeyboard(
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("1"),
				tgbotapi.NewKeyboardButton("2"),
				tgbotapi.NewKeyboardButton("3"),
			),
		)
		keyboard.ResizeKeyboard = true
		out := tgbotapi.NewMessage(msg.Chat.ID, "How are you? Choose 1, 2 or 3:")
		out.ReplyMarkup = keyboard
		if _, err := sender.Send(out); err != nil {
			log.Printf("survey: send question to chat=%d: %v", msg.Chat.ID, err)
		}
	}
}

// AnswerPredicate returns true when the user is awaiting a howami answer.
func AnswerPredicate(store *state.Store) router.TextPredicate {
	return func(msg *tgbotapi.Message) bool {
		if msg.From == nil {
			return false
		}
		return store.Get(msg.From.ID).State == state.StateAwaitingHowamiAnswer
	}
}

// AnswerHandler persists a valid answer (1/2/3) and resets state.
// Invalid input prompts the user to try again without changing state.
func AnswerHandler(store *state.Store) router.HandlerFunc {
	return func(sender router.Sender, msg *tgbotapi.Message) {
		n, err := strconv.Atoi(msg.Text)
		if err != nil || n < 1 || n > 3 {
			reply(sender, msg, "Please choose 1, 2 or 3.")
			return
		}

		d := store.Get(msg.From.ID)
		d.HowamiAnswer = n
		d.State = state.StateIdle
		store.Set(msg.From.ID, d)

		reply(sender, msg, fmt.Sprintf("Got it — you answered %d. Thanks!", n))
	}
}

func reply(sender router.Sender, msg *tgbotapi.Message, text string) {
	out := tgbotapi.NewMessage(msg.Chat.ID, text)
	out.ReplyToMessageID = msg.MessageID
	sender.Send(out)
}
