package bot

import (
	"context"
	"fmt"
	"log"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/padington/tgbase/internal/flows/meta"
	"github.com/padington/tgbase/internal/flows/survey"
	"github.com/padington/tgbase/internal/reminder"
	"github.com/padington/tgbase/internal/router"
	"github.com/padington/tgbase/internal/state"
)

type Config struct {
	Token              string
	Debug              bool
	Timeout            int
	Env                string
	DataPath           string
	StateFlushInterval time.Duration
	Reminder           reminder.Config
}

type Bot struct {
	api      *tgbotapi.BotAPI
	cfg      Config
	router   *router.Router
	store    *state.Store
	reminder *reminder.Worker
}

func New(cfg Config) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(cfg.Token)
	if err != nil {
		return nil, fmt.Errorf("create bot api: %w", err)
	}
	api.Debug = cfg.Debug
	log.Printf("authorised as @%s", api.Self.UserName)

	persister := buildPersister(cfg)
	store := state.NewStore(persister)
	r := router.New(api)

	r.HandleCommand("ping", meta.Ping())
	r.HandleCommand("whoami", meta.Whoami(cfg.Env))
	r.HandleCommand("menu", meta.Menu())
	r.HandleText(exactText("Ping"), meta.Ping())
	r.HandleText(exactText("Whoami"), meta.Whoami(cfg.Env))

	r.HandleCommand("start", survey.Start(store))
	r.HandleText(survey.AnswerPredicate(store), survey.AnswerHandler(store))

	worker := reminder.New(store, api, cfg.Reminder)

	return &Bot{api: api, cfg: cfg, router: r, store: store, reminder: worker}, nil
}

func (b *Bot) Run(ctx context.Context) error {
	defer func() {
		if err := b.store.Close(); err != nil {
			log.Printf("bot: store close: %v", err)
		}
	}()

	go b.reminder.Run(ctx)

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

func buildPersister(cfg Config) state.Persister {
	if cfg.DataPath == "" {
		log.Println("bot: no DataPath configured, using in-memory state")
		return state.MemoryPersister{}
	}
	log.Printf("bot: persisting state to %s (flush interval %s)", cfg.DataPath, cfg.StateFlushInterval)
	file := state.NewFilePersister(cfg.DataPath)
	if cfg.StateFlushInterval <= 0 {
		return file
	}
	return state.NewDebouncedPersister(file, cfg.StateFlushInterval)
}

func exactText(s string) router.TextPredicate {
	return func(msg *tgbotapi.Message) bool { return msg.Text == s }
}
