package trader

import (
	"nofx/logger"
	"nofx/market"
	"nofx/store"
	"sync"
	"time"
)

// LimitOrderEngine monitors pending limit orders and triggers execution.
type LimitOrderEngine struct {
	store        *store.Store
	traders      map[string]*AutoTrader // traderID -> AutoTrader
	tradersMu    sync.RWMutex
	stopCh       chan struct{}
	wg           sync.WaitGroup
	checkInterval time.Duration
}

// NewLimitOrderEngine creates a new limit order engine.
func NewLimitOrderEngine(st *store.Store) *LimitOrderEngine {
	return &LimitOrderEngine{
		store:         st,
		traders:       make(map[string]*AutoTrader),
		checkInterval: 500 * time.Millisecond,
	}
}

// RegisterTrader registers a trader for limit order execution.
func (e *LimitOrderEngine) RegisterTrader(traderID string, at *AutoTrader) {
	e.tradersMu.Lock()
	defer e.tradersMu.Unlock()
	e.traders[traderID] = at
	logger.Infof("[LimitOrderEngine] Registered trader: %s", traderID)
}

// UnregisterTrader removes a trader from the engine.
func (e *LimitOrderEngine) UnregisterTrader(traderID string) {
	e.tradersMu.Lock()
	defer e.tradersMu.Unlock()
	delete(e.traders, traderID)
	logger.Infof("[LimitOrderEngine] Unregistered trader: %s", traderID)
}

// Start begins the limit order monitoring loop.
func (e *LimitOrderEngine) Start() {
	e.stopCh = make(chan struct{})
	e.wg.Add(1)
	go e.monitorLoop()
	logger.Info("[LimitOrderEngine] Started")
}

// Stop halts the limit order monitoring.
func (e *LimitOrderEngine) Stop() {
	if e.stopCh != nil {
		close(e.stopCh)
		e.wg.Wait()
	}
	logger.Info("[LimitOrderEngine] Stopped")
}

func (e *LimitOrderEngine) monitorLoop() {
	defer e.wg.Done()
	ticker := time.NewTicker(e.checkInterval)
	defer ticker.Stop()

	// Expire stale orders every minute
	expireTicker := time.NewTicker(1 * time.Minute)
	defer expireTicker.Stop()

	for {
		select {
		case <-e.stopCh:
			return
		case <-ticker.C:
			e.checkPendingOrders()
		case <-expireTicker.C:
			if expired, err := e.store.LimitOrder().ExpireLimitOrders(); err != nil {
				logger.Warnf("[LimitOrderEngine] Failed to expire orders: %v", err)
			} else if expired > 0 {
				logger.Infof("[LimitOrderEngine] Expired %d orders", expired)
			}
		}
	}
}

func (e *LimitOrderEngine) checkPendingOrders() {
	orders, err := e.store.LimitOrder().GetAllPendingOrders()
	if err != nil {
		logger.Warnf("[LimitOrderEngine] Failed to get pending orders: %v", err)
		return
	}
	if len(orders) == 0 {
		return
	}

	for _, order := range orders {
		marketData, err := market.Get(order.Symbol)
		if err != nil {
			logger.Warnf("[LimitOrderEngine] Failed to get price for %s: %v", order.Symbol, err)
			continue
		}
		currentPrice := marketData.CurrentPrice

		triggered := false
		switch order.TriggerCondition {
		case "lte":
			triggered = currentPrice <= order.TriggerPrice
		case "gte":
			triggered = currentPrice >= order.TriggerPrice
		}

		if triggered {
			e.executeOrder(&order, currentPrice)
		}
	}
}

func (e *LimitOrderEngine) executeOrder(order *store.LimitOrderModel, price float64) {
	e.tradersMu.RLock()
	at, ok := e.traders[order.TraderID]
	e.tradersMu.RUnlock()
	if !ok {
		logger.Warnf("[LimitOrderEngine] Trader %s not registered, canceling order %s", order.TraderID, order.ID)
		e.store.LimitOrder().CancelLimitOrder(order.ID)
		return
	}

	trader := at.GetUnderlyingTrader()
	var result map[string]interface{}
	var execErr error

	switch order.Side {
	case "BUY":
		if order.PositionSide == "LONG" {
			result, execErr = trader.OpenLong(order.Symbol, order.Quantity, order.Leverage)
		} else {
			result, execErr = trader.CloseShort(order.Symbol, order.Quantity)
		}
	case "SELL":
		if order.PositionSide == "SHORT" {
			result, execErr = trader.OpenShort(order.Symbol, order.Quantity, order.Leverage)
		} else {
			result, execErr = trader.CloseLong(order.Symbol, order.Quantity)
		}
	}

	now := time.Now().UTC()
	if execErr != nil {
		logger.Errorf("[LimitOrderEngine] Failed to execute order %s: %v", order.ID, execErr)
		e.store.LimitOrder().UpdateLimitOrderStatus(order.ID, "failed", 0, 0, 0)
		return
	}

	filledPrice := price
	filledQty := order.Quantity
	var commission float64
	var exchangeOrderID string

	if result != nil {
		if avg, ok := result["avgPrice"].(float64); ok && avg > 0 {
			filledPrice = avg
		}
		if qty, ok := result["executedQty"].(float64); ok && qty > 0 {
			filledQty = qty
		}
		if fee, ok := result["commission"].(float64); ok {
			commission = fee
		}
		if oid, ok := result["orderId"].(string); ok {
			exchangeOrderID = oid
		}
	}

	e.store.LimitOrder().UpdateLimitOrderStatus(order.ID, "filled", filledPrice, filledQty, commission)

	// Update triggered_at via direct update
	e.store.GormDB().Model(&store.LimitOrderModel{}).Where("id = ?", order.ID).Updates(map[string]interface{}{
		"triggered_at":      now,
		"exchange_order_id": exchangeOrderID,
	})

	logger.Infof("[LimitOrderEngine] Order %s filled: %s %s %.4f @ %.4f",
		order.ID, order.Side, order.Symbol, filledQty, filledPrice)
}
