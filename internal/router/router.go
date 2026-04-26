package router

import (
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Sender is the only surface handlers use to speak to Telegram.
type Sender interface {
	Send(c tgbotapi.Chattable) (tgbotapi.Message, error)
}

// HandlerFunc is the signature every handler must satisfy.
type HandlerFunc func(sender Sender, msg *tgbotapi.Message)

// TextPredicate decides whether a text handler should handle a message.
type TextPredicate func(msg *tgbotapi.Message) bool

type textEntry struct {
	match   TextPredicate
	handler HandlerFunc
}

// Router dispatches incoming updates to registered handlers.
type Router struct {
	sender   Sender
	commands map[string]HandlerFunc
	texts    []textEntry
}

func New(sender Sender) *Router {
	return &Router{
		sender:   sender,
		commands: make(map[string]HandlerFunc),
	}
}

// HandleCommand registers a handler for the given command (without "/").
func (r *Router) HandleCommand(cmd string, h HandlerFunc) {
	r.commands[cmd] = h
}

// HandleText registers a handler that fires when pred(msg) is true.
// Registrations are tried in order; first match wins.
func (r *Router) HandleText(pred TextPredicate, h HandlerFunc) {
	r.texts = append(r.texts, textEntry{pred, h})
}

// Dispatch routes a single update to the appropriate handler.
func (r *Router) Dispatch(update tgbotapi.Update) {
	if update.Message == nil {
		log.Printf("skipping non-message update id=%d", update.UpdateID)
		return
	}
	msg := update.Message

	if msg.IsCommand() {
		cmd := msg.Command()
		h, ok := r.commands[cmd]
		if !ok {
			log.Printf("unknown command: /%s", cmd)
			return
		}
		log.Printf("command /%s from @%s (chat=%d)", cmd, msg.From.UserName, msg.Chat.ID)
		h(r.sender, msg)
		return
	}

	for _, e := range r.texts {
		if e.match(msg) {
			log.Printf("text %q from @%s (chat=%d)", msg.Text, msg.From.UserName, msg.Chat.ID)
			e.handler(r.sender, msg)
			return
		}
	}
	log.Printf("unhandled text from @%s: %q", msg.From.UserName, msg.Text)
}
