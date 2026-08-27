package bot

import (
	"errors"
	"strings"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/padington/tgbase/internal/i18n"
)

// realTrans loads the REAL bundled i18n yamls (../../i18n) with ru as the
// default locale — same as bot.New — so these tests also pin that every menu
// description key actually exists in the shipped bundles.
func realTrans(t *testing.T) i18n.Translator {
	t.Helper()
	trans, err := i18n.Load("../../i18n", "ru")
	if err != nil {
		t.Fatalf("load bundled i18n: %v", err)
	}
	return trans
}

// allModes is the fully-wired binary: every self-check track registered.
var allModes = modes{screening: true, mood: true, eating: true}

func commandNames(cmds []tgbotapi.BotCommand) []string {
	names := make([]string, len(cmds))
	for i, c := range cmds {
		names[i] = c.Command
	}
	return names
}

func TestMenuCommands_FullListInFrequencyOrder(t *testing.T) {
	cmds := menuCommands(realTrans(t), "", allModes)

	want := []string{
		"menu", "adhd", "mood", "food",
		"report", "about", "abandon",
		"adhd_delete", "mood_delete", "food_delete",
	}
	got := commandNames(cmds)
	if len(got) != len(want) {
		t.Fatalf("expected %d commands, got %d: %v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("command %d: expected %q, got %q (full: %v)", i, want[i], got[i], got)
		}
	}
}

func TestMenuCommands_ServiceCommandsExcluded(t *testing.T) {
	cmds := menuCommands(realTrans(t), "", allModes)
	for _, c := range cmds {
		switch c.Command {
		case "start", "ping", "whoami":
			t.Fatalf("/%s must not appear in the command menu", c.Command)
		}
	}
}

// A track that is not wired must contribute NEITHER of its two commands: the
// menu is the user's contract with the running binary, and an entry the
// router never registered is a dead command.
func TestMenuCommands_ModesDisabled(t *testing.T) {
	trans := realTrans(t)

	cases := []struct {
		name  string
		modes modes
		want  []string
	}{
		{
			"screening off",
			modes{screening: false, mood: true, eating: true},
			[]string{"menu", "mood", "food", "report", "about", "abandon", "mood_delete", "food_delete"},
		},
		{
			"mood off",
			modes{screening: true, mood: false, eating: true},
			[]string{"menu", "adhd", "food", "report", "about", "abandon", "adhd_delete", "food_delete"},
		},
		{
			"eating off",
			modes{screening: true, mood: true, eating: false},
			[]string{"menu", "adhd", "mood", "report", "about", "abandon", "adhd_delete", "mood_delete"},
		},
		{
			"all tracks off",
			modes{},
			[]string{"menu", "report", "about", "abandon"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := commandNames(menuCommands(trans, "", tc.modes))
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("expected %v, got %v", tc.want, got)
			}
		})
	}
}

// Every description must resolve from the bundled i18n: a lookup miss returns
// the key verbatim, which both breaks the UX and (>256 chars aside) is the
// canary for a forgotten yaml entry.
func TestMenuCommands_DescriptionsResolveFromI18n(t *testing.T) {
	trans := realTrans(t)
	for _, locale := range []i18n.Locale{"", "en"} {
		for _, c := range menuCommands(trans, locale, allModes) {
			if c.Description == "" || strings.HasPrefix(c.Description, "cmd.") {
				t.Errorf("locale %q: /%s description did not resolve: %q", locale, c.Command, c.Description)
			}
			if n := len(c.Description); n < 3 || n > 256 {
				t.Errorf("locale %q: /%s description length %d outside Telegram's 3..256", locale, c.Command, n)
			}
		}
	}
}

func TestMenuCommands_LocalizedDescriptions(t *testing.T) {
	trans := realTrans(t)

	ru := menuCommands(trans, "", allModes)
	if ru[0].Description != "главное меню" {
		t.Fatalf("default-locale /menu description: got %q", ru[0].Description)
	}

	en := menuCommands(trans, "en", allModes)
	if en[0].Description != "main menu" {
		t.Fatalf("en /menu description: got %q", en[0].Description)
	}
}

func TestMenuConfigs_DefaultPlusEnglish(t *testing.T) {
	cfgs := menuConfigs(realTrans(t), allModes)
	if len(cfgs) != 2 {
		t.Fatalf("expected 2 setMyCommands configs (default + en), got %d", len(cfgs))
	}
	if cfgs[0].LanguageCode != "" || cfgs[0].Scope != nil {
		t.Fatalf("first config must be the plain default scope, got lang=%q scope=%v", cfgs[0].LanguageCode, cfgs[0].Scope)
	}
	if cfgs[1].LanguageCode != "en" {
		t.Fatalf("second config must target language_code=en, got %q", cfgs[1].LanguageCode)
	}
	if cfgs[1].Scope == nil || cfgs[1].Scope.Type != "default" {
		t.Fatalf("en config must use the default scope, got %+v", cfgs[1].Scope)
	}
	if len(cfgs[0].Commands) != len(cfgs[1].Commands) {
		t.Fatalf("locale variants must list the same commands: %d vs %d", len(cfgs[0].Commands), len(cfgs[1].Commands))
	}
}

// mockRegistrar records every Request call and can be told to fail.
type mockRegistrar struct {
	requests []tgbotapi.Chattable
	err      error
}

func (m *mockRegistrar) Request(c tgbotapi.Chattable) (*tgbotapi.APIResponse, error) {
	m.requests = append(m.requests, c)
	if m.err != nil {
		return nil, m.err
	}
	return &tgbotapi.APIResponse{Ok: true}, nil
}

func TestRegisterCommands_SendsBothLocaleVariants(t *testing.T) {
	reg := &mockRegistrar{}
	registerCommands(reg, realTrans(t), allModes)

	if len(reg.requests) != 2 {
		t.Fatalf("expected 2 setMyCommands requests, got %d", len(reg.requests))
	}
	for i, req := range reg.requests {
		cfg, ok := req.(tgbotapi.SetMyCommandsConfig)
		if !ok {
			t.Fatalf("request %d: expected SetMyCommandsConfig, got %T", i, req)
		}
		if len(cfg.Commands) == 0 {
			t.Fatalf("request %d: empty command list", i)
		}
	}
}

func TestRegisterCommands_APIFailureIsNotFatal(t *testing.T) {
	reg := &mockRegistrar{err: errors.New("telegram is down")}

	// Must neither panic nor abort after the first failure.
	registerCommands(reg, realTrans(t), allModes)

	if len(reg.requests) != 2 {
		t.Fatalf("expected registration to attempt all configs despite errors, got %d", len(reg.requests))
	}
}
