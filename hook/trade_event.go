package hook

import "time"

type TradeEvent struct {
	TraderID    string
	TraderName  string
	UserID      string
	Exchange    string
	ExchangeID  string
	Symbol      string
	Action      string // open_long, open_short, close_long, close_short
	Side        string // LONG, SHORT
	Quantity    float64
	Price       float64
	EntryPrice  float64 // close order entry price
	RealizedPnL float64 // close order realized PnL
	Fee         float64
	Leverage    int
	OrderID     string
	TradeID     string
	Timestamp   time.Time
	Source      string // "auto" | "sync" | "grid"
}

type TradeEventResult struct {
	Err error
}

func (r *TradeEventResult) Error() error    { return r.Err }
func (r *TradeEventResult) GetResult() error { return r.Err }
