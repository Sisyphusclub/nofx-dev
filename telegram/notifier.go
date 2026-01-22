package telegram

import (
	"context"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"golang.org/x/time/rate"
)

type Notifier struct {
	api     *tgbotapi.BotAPI
	limiter *rate.Limiter
}

func NewNotifier(api *tgbotapi.BotAPI) *Notifier {
	return &Notifier{
		api:     api,
		limiter: rate.NewLimiter(rate.Every(time.Second/20), 5), // 20/sec burst 5
	}
}

func (n *Notifier) Send(chatID int64, message string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := n.limiter.Wait(ctx); err != nil {
		return err
	}

	msg := tgbotapi.NewMessage(chatID, message)
	_, err := n.api.Send(msg)
	return err
}

func (n *Notifier) SendMarkdown(chatID int64, message string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := n.limiter.Wait(ctx); err != nil {
		return err
	}

	msg := tgbotapi.NewMessage(chatID, message)
	msg.ParseMode = tgbotapi.ModeMarkdown
	_, err := n.api.Send(msg)
	return err
}
