package telegram

import (
	"nofx/hook"
	"nofx/logger"
	"nofx/store"
	"strings"
)

var globalBot *Bot

func Init(cfg *Config, st *store.Store) error {
	if !cfg.Enabled {
		logger.Infof("[Telegram] Bot disabled (no token configured)")
		return nil
	}

	bot, err := NewBot(cfg, st)
	if err != nil {
		return err
	}
	if bot == nil {
		return nil
	}

	globalBot = bot
	registerHooks(bot, st)
	bot.Start()

	return nil
}

func GetBot() *Bot {
	return globalBot
}

func Shutdown() {
	if globalBot != nil {
		globalBot.Stop()
		globalBot = nil
	}
}

func registerHooks(bot *Bot, st *store.Store) {
	hook.RegisterHook(hook.TRADE_EXECUTED, func(args ...any) any {
		if len(args) == 0 {
			return &hook.TradeEventResult{}
		}

		event, ok := args[0].(*hook.TradeEvent)
		if !ok || event == nil {
			return &hook.TradeEventResult{}
		}

		go handleTradeEvent(bot, st, event)
		return &hook.TradeEventResult{}
	})

	logger.Infof("[Telegram] Registered TRADE_EXECUTED hook")
}

func handleTradeEvent(bot *Bot, st *store.Store, event *hook.TradeEvent) {
	settings, err := st.Telegram().GetByTraderID(event.TraderID)
	if err != nil {
		logger.Warnf("[Telegram] Failed to get settings for trader %s: %v", event.TraderID, err)
		return
	}

	if settings == nil || !settings.Enabled {
		return
	}

	if settings.ChatID == 0 {
		if bot.config.DefaultChatID == 0 {
			return
		}
		settings.ChatID = bot.config.DefaultChatID
	}

	isOpen := strings.HasPrefix(event.Action, "open")
	isClose := strings.HasPrefix(event.Action, "close")

	if isOpen && !settings.NotifyOpen {
		return
	}
	if isClose && !settings.NotifyClose {
		return
	}

	var msg string
	if isOpen {
		msg = RenderOpenPosition(event)
	} else if isClose {
		msg = RenderClosePosition(event)
	} else {
		return
	}

	if err := bot.notifier.SendMarkdown(settings.ChatID, msg); err != nil {
		logger.Warnf("[Telegram] Failed to send notification: %v", err)
	}
}
