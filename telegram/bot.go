package telegram

import (
	"nofx/logger"
	"nofx/store"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Bot struct {
	api      *tgbotapi.BotAPI
	store    *store.Store
	notifier *Notifier
	config   *Config
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

func NewBot(cfg *Config, st *store.Store) (*Bot, error) {
	if !cfg.Enabled || cfg.BotToken == "" {
		return nil, nil
	}

	api, err := tgbotapi.NewBotAPI(cfg.BotToken)
	if err != nil {
		return nil, err
	}

	bot := &Bot{
		api:    api,
		store:  st,
		config: cfg,
		stopCh: make(chan struct{}),
	}
	bot.notifier = NewNotifier(api)

	logger.Infof("[Telegram] Bot authorized as @%s", api.Self.UserName)
	return bot, nil
}

func (b *Bot) Start() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)

	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		for {
			select {
			case <-b.stopCh:
				return
			case update := <-updates:
				if update.Message != nil && update.Message.IsCommand() {
					b.handleCommand(update.Message)
				}
			}
		}
	}()

	logger.Infof("[Telegram] Bot started, listening for commands...")
}

func (b *Bot) Stop() {
	close(b.stopCh)
	b.api.StopReceivingUpdates()
	b.wg.Wait()
	logger.Infof("[Telegram] Bot stopped")
}

func (b *Bot) Notifier() *Notifier {
	return b.notifier
}

func (b *Bot) Store() *store.Store {
	return b.store
}

func (b *Bot) Config() *Config {
	return b.config
}
