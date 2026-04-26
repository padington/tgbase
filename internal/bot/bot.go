package bot

import (
	"context"
	"fmt"
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Config struct {
	Token   string
	Debug   bool
	Timeout int
}

type Bot struct {
	api *tgbotapi.BotAPI
	cfg Config
}

func New(cfg Config) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(cfg.Token)
	if err != nil {
		return nil, fmt.Errorf("create bot api: %w", err)
	}
	api.Debug = cfg.Debug
	log.Printf("authorised as @%s", api.Self.UserName)
	return &Bot{api: api, cfg: cfg}, nil
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
			b.handleUpdate(update)
		}
	}
}
