package bot

import (
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/padington/tgbase/internal/i18n"
)

// commandRegistrar is the narrow slice of *tgbotapi.BotAPI needed to register
// the command menu; kept as an interface so tests can capture the calls.
type commandRegistrar interface {
	Request(c tgbotapi.Chattable) (*tgbotapi.APIResponse, error)
}

// menuCommands returns the client command menu in usage-frequency order.
// Descriptions come from i18n ("cmd.<name>.desc"). Deliberate omissions:
// /start (Telegram renders its own Start button) and the operator commands
// /ping and /whoami. adhd/mood entries appear only when the corresponding
// mode is actually wired, so the menu always matches the running binary.
func menuCommands(trans i18n.Translator, locale i18n.Locale, screening, mood bool) []tgbotapi.BotCommand {
	entries := []struct {
		name    string
		enabled bool
	}{
		{"menu", true},
		{"adhd", screening},
		{"mood", mood},
		{"report", true},
		{"about", true},
		{"abandon", true},
		{"adhd_delete", screening},
		{"mood_delete", mood},
	}
	var out []tgbotapi.BotCommand
	for _, e := range entries {
		if !e.enabled {
			continue
		}
		out = append(out, tgbotapi.BotCommand{
			Command:     e.name,
			Description: trans.T("cmd."+e.name+".desc", locale, nil),
		})
	}
	return out
}

// menuConfigs builds the setMyCommands payloads: the default-scope list with
// default-locale (ru) descriptions, plus a language_code="en" variant that
// overrides it for clients running in English.
func menuConfigs(trans i18n.Translator, screening, mood bool) []tgbotapi.SetMyCommandsConfig {
	return []tgbotapi.SetMyCommandsConfig{
		tgbotapi.NewSetMyCommands(menuCommands(trans, "", screening, mood)...),
		tgbotapi.NewSetMyCommandsWithScopeAndLanguage(
			tgbotapi.NewBotCommandScopeDefault(), "en",
			menuCommands(trans, "en", screening, mood)...),
	}
}

// registerCommands self-registers the command menu on boot so the client
// hint never drifts from the deployed binary (no manual BotFather step).
// Failures are logged as warnings and never fatal: a broken setMyCommands
// call must not stop the bot from serving updates.
func registerCommands(api commandRegistrar, trans i18n.Translator, screening, mood bool) {
	for _, cfg := range menuConfigs(trans, screening, mood) {
		if _, err := api.Request(cfg); err != nil {
			log.Printf("bot: warning: setMyCommands (lang=%q) failed, command menu may be stale: %v", cfg.LanguageCode, err)
		}
	}
}
