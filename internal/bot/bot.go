package bot

import (
	"context"
	"fmt"
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/padington/tgbase/internal/flows/meta"
	"github.com/padington/tgbase/internal/flows/survey"
	"github.com/padington/tgbase/internal/router"
	"github.com/padington/tgbase/internal/state"
)

type Config struct {
	Token   string
	Debug   bool
	Timeout int
	Env     string
}

type Bot struct {
	api    *tgbotapi.BotAPI
	cfg    Config
	router *router.Router
}

func New(cfg Config) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(cfg.Token)
	if err != nil {
		return nil, fmt.Errorf("create bot api: %w", err)
	}
	api.Debug = cfg.Debug
	log.Printf("authorised as @%s", api.Self.UserName)

	store := state.NewStore()
	r := router.New(api)

	// meta — stateless healthcheck commands and their button-tap equivalents
	r.HandleCommand("ping", meta.Ping())
	r.HandleCommand("whoami", meta.Whoami(cfg.Env))
	r.HandleCommand("menu", meta.Menu())
	r.HandleText(exactText("Ping"), meta.Ping())
	r.HandleText(exactText("Whoami"), meta.Whoami(cfg.Env))

	// survey flow — stateful /start conversation
	r.HandleCommand("start", survey.Start(store))
	r.HandleText(survey.AnswerPredicate(store), survey.AnswerHandler(store))

	return &Bot{api: api, cfg: cfg, router: r}, nil
}

func (b *Bot) Run(ctx context.Context) error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = b.cfg.Timeout

	updates := b.api.GetUpdatesChan(u)
	log.Printf("polling for updates (timeout=%ds)", b.cfg.Timeout)

	for {
		select {
		case <-ctx.Done():
			log.Println("shutdown signal received, stopping updates")
			b.api.StopReceivingUpdates()
			return nil
		case update := <-updates:
			log.Printf("update id=%d", update.UpdateID)
			b.router.Dispatch(update)
		}
	}
}

func exactText(s string) router.TextPredicate {
	return func(msg *tgbotapi.Message) bool { return msg.Text == s }
}
