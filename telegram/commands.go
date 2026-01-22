package telegram

import (
	"fmt"
	"nofx/logger"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (b *Bot) handleCommand(msg *tgbotapi.Message) {
	chatID := msg.Chat.ID
	cmd := msg.Command()
	args := strings.Fields(msg.CommandArguments())

	logger.Infof("[Telegram] Received command: /%s from chat %d", cmd, chatID)

	switch cmd {
	case "start":
		b.handleStart(chatID)
	case "help":
		b.handleHelp(chatID)
	case "balance":
		b.handleBalance(chatID, args)
	case "pnl":
		b.handlePnL(chatID, args)
	case "status":
		b.handleStatus(chatID, args)
	case "traders":
		b.handleTraders(chatID)
	default:
		b.notifier.Send(chatID, fmt.Sprintf("Unknown command: /%s\nUse /help for available commands.", cmd))
	}
}

func (b *Bot) handleStart(chatID int64) {
	msg := `🤖 *NOFX AI Trader Bot*
━━━━━━━━━━━━━━━━
Welcome! I'll notify you about trading activities.

Your Chat ID: ` + fmt.Sprintf("`%d`", chatID) + `

Use this Chat ID in NOFX settings to receive notifications.

Use /help to see available commands.`
	b.notifier.SendMarkdown(chatID, msg)
}

func (b *Bot) handleHelp(chatID int64) {
	msg := `📖 *Available Commands*
━━━━━━━━━━━━━━━━
/balance [trader] - Check account balance
/pnl [trader] - Check realized PnL
/status [trader] - Check trader status
/traders - List all traders
/help - Show this help message

_Specify trader ID or leave empty for default._`
	b.notifier.SendMarkdown(chatID, msg)
}

func (b *Bot) handleBalance(chatID int64, args []string) {
	traderID := ""
	if len(args) > 0 {
		traderID = args[0]
	}

	traders, err := b.store.Trader().ListAll()
	if err != nil {
		b.notifier.Send(chatID, "Failed to get traders: "+err.Error())
		return
	}

	if len(traders) == 0 {
		b.notifier.Send(chatID, "No traders configured.")
		return
	}

	var target *struct {
		ID   string
		Name string
	}

	for _, t := range traders {
		if traderID == "" || t.ID == traderID || t.Name == traderID {
			target = &struct {
				ID   string
				Name string
			}{ID: t.ID, Name: t.Name}
			break
		}
	}

	if target == nil {
		b.notifier.Send(chatID, "Trader not found. Use /traders to list available traders.")
		return
	}

	b.notifier.SendMarkdown(chatID, fmt.Sprintf(`💰 *Balance Query*
━━━━━━━━━━━━━━━━
Trader: %s
_Balance queries require live exchange connection._
_Check the NOFX dashboard for real-time data._`, target.Name))
}

func (b *Bot) handlePnL(chatID int64, args []string) {
	traderID := ""
	if len(args) > 0 {
		traderID = args[0]
	}

	traders, err := b.store.Trader().ListAll()
	if err != nil {
		b.notifier.Send(chatID, "Failed to get traders: "+err.Error())
		return
	}

	if len(traders) == 0 {
		b.notifier.Send(chatID, "No traders configured.")
		return
	}

	var targetName string
	for _, t := range traders {
		if traderID == "" || t.ID == traderID || t.Name == traderID {
			targetName = t.Name
			break
		}
	}

	if targetName == "" {
		b.notifier.Send(chatID, "Trader not found. Use /traders to list available traders.")
		return
	}

	b.notifier.SendMarkdown(chatID, fmt.Sprintf(`📊 *PnL Query*
━━━━━━━━━━━━━━━━
Trader: %s
_PnL queries require live exchange connection._
_Check the NOFX dashboard for real-time data._`, targetName))
}

func (b *Bot) handleStatus(chatID int64, args []string) {
	traderID := ""
	if len(args) > 0 {
		traderID = args[0]
	}

	traders, err := b.store.Trader().ListAll()
	if err != nil {
		b.notifier.Send(chatID, "Failed to get traders: "+err.Error())
		return
	}

	if len(traders) == 0 {
		b.notifier.Send(chatID, "No traders configured.")
		return
	}

	for _, t := range traders {
		if traderID == "" || t.ID == traderID || t.Name == traderID {
			status := "🔴 Stopped"
			if t.IsRunning {
				status = "🟢 Running"
			}

			msg := fmt.Sprintf(`📋 *Trader Status*
━━━━━━━━━━━━━━━━
Name: %s
ID: `+"`%s`"+`
Status: %s
Symbols: %s
Leverage: BTC/ETH %dx, Alt %dx`, t.Name, t.ID, status, t.TradingSymbols, t.BTCETHLeverage, t.AltcoinLeverage)
			b.notifier.SendMarkdown(chatID, msg)
			return
		}
	}

	b.notifier.Send(chatID, "Trader not found. Use /traders to list available traders.")
}

func (b *Bot) handleTraders(chatID int64) {
	traders, err := b.store.Trader().ListAll()
	if err != nil {
		b.notifier.Send(chatID, "Failed to get traders: "+err.Error())
		return
	}

	if len(traders) == 0 {
		b.notifier.Send(chatID, "No traders configured.")
		return
	}

	var sb strings.Builder
	sb.WriteString("📋 *Configured Traders*\n━━━━━━━━━━━━━━━━\n")

	for _, t := range traders {
		status := "🔴"
		if t.IsRunning {
			status = "🟢"
		}
		sb.WriteString(fmt.Sprintf("%s `%s` - %s\n", status, t.ID[:8], t.Name))
	}

	b.notifier.SendMarkdown(chatID, sb.String())
}
