package telegram

import (
	"fmt"
	"nofx/hook"
	"strings"
)

func RenderOpenPosition(e *hook.TradeEvent) string {
	emoji := "📈"
	direction := "做多"
	if strings.Contains(e.Action, "short") {
		emoji = "📉"
		direction = "做空"
	}

	return fmt.Sprintf(`%s *%s | %s*
━━━━━━━━━━━━━━━━
📍 交易对: `+"`%s`"+`
🎯 方向: *%s*
💰 价格: $%.4f
📦 数量: %.4f
⚡ 杠杆: %dx
━━━━━━━━━━━━━━━━
_NOFX AI 交易员_`,
		emoji, e.TraderName, e.Exchange,
		e.Symbol,
		direction,
		e.Price,
		e.Quantity,
		e.Leverage,
	)
}

func RenderClosePosition(e *hook.TradeEvent) string {
	emoji := "✅"
	pnlEmoji := "💚"
	pnlSign := "+"
	direction := "平多"
	if e.RealizedPnL < 0 {
		emoji = "❌"
		pnlEmoji = "💔"
		pnlSign = ""
	}
	if strings.Contains(e.Action, "short") {
		direction = "平空"
	}

	pnlPercent := 0.0
	if e.EntryPrice > 0 && e.Quantity > 0 {
		cost := e.EntryPrice * e.Quantity
		pnlPercent = (e.RealizedPnL / cost) * 100
	}

	return fmt.Sprintf(`%s *%s | %s*
━━━━━━━━━━━━━━━━
📍 交易对: `+"`%s`"+`
🎯 方向: *%s*
📥 开仓价: $%.4f
📤 平仓价: $%.4f
%s 盈亏: *%s$%.2f* (%.2f%%)
━━━━━━━━━━━━━━━━
_NOFX AI 交易员_`,
		emoji, e.TraderName, e.Exchange,
		e.Symbol,
		direction,
		e.EntryPrice,
		e.Price,
		pnlEmoji, pnlSign, e.RealizedPnL, pnlPercent,
	)
}

func RenderError(traderName, message string) string {
	return fmt.Sprintf(`⚠️ *%s | 错误*
━━━━━━━━━━━━━━━━
%s
━━━━━━━━━━━━━━━━
_NOFX AI 交易员_`,
		traderName, message,
	)
}
