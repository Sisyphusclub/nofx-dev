package trader

import (
	"nofx/logger"
	"nofx/market"
	"nofx/store"
	"sync"
	"time"
)

// TrailingStopTracker monitors positions and adjusts trailing stop prices.
type TrailingStopTracker struct {
	store        *store.Store
	traders      map[string]*AutoTrader
	tradersMu    sync.RWMutex
	stopCh       chan struct{}
	wg           sync.WaitGroup
	checkInterval time.Duration
}

// NewTrailingStopTracker creates a new trailing stop tracker.
func NewTrailingStopTracker(st *store.Store) *TrailingStopTracker {
	return &TrailingStopTracker{
		store:         st,
		traders:       make(map[string]*AutoTrader),
		checkInterval: 1 * time.Second,
	}
}

// RegisterTrader registers a trader for trailing stop monitoring.
func (t *TrailingStopTracker) RegisterTrader(traderID string, at *AutoTrader) {
	t.tradersMu.Lock()
	defer t.tradersMu.Unlock()
	t.traders[traderID] = at
	logger.Infof("[TrailingStop] Registered trader: %s", traderID)
}

// UnregisterTrader removes a trader from tracking.
func (t *TrailingStopTracker) UnregisterTrader(traderID string) {
	t.tradersMu.Lock()
	defer t.tradersMu.Unlock()
	delete(t.traders, traderID)
	logger.Infof("[TrailingStop] Unregistered trader: %s", traderID)
}

// Start begins the trailing stop monitoring loop.
func (t *TrailingStopTracker) Start() {
	t.stopCh = make(chan struct{})
	t.wg.Add(1)
	go t.monitorLoop()
	logger.Info("[TrailingStop] Tracker started")
}

// Stop halts the trailing stop monitoring.
func (t *TrailingStopTracker) Stop() {
	if t.stopCh != nil {
		close(t.stopCh)
		t.wg.Wait()
	}
	logger.Info("[TrailingStop] Tracker stopped")
}

func (t *TrailingStopTracker) monitorLoop() {
	defer t.wg.Done()
	ticker := time.NewTicker(t.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-t.stopCh:
			return
		case <-ticker.C:
			t.checkActiveStops()
		}
	}
}

func (t *TrailingStopTracker) checkActiveStops() {
	configs, err := t.store.TrailingStop().GetAllActiveTrailingStops()
	if err != nil {
		logger.Warnf("[TrailingStop] Failed to get active configs: %v", err)
		return
	}
	if len(configs) == 0 {
		return
	}

	for _, cfg := range configs {
		marketData, err := market.Get(cfg.Symbol)
		if err != nil {
			logger.Warnf("[TrailingStop] Failed to get price for %s: %v", cfg.Symbol, err)
			continue
		}
		currentPrice := marketData.CurrentPrice

		t.processConfig(&cfg, currentPrice)
	}
}

func (t *TrailingStopTracker) processConfig(cfg *store.TrailingStopConfigModel, currentPrice float64) {
	// Check activation price
	if cfg.ActivationPrice > 0 {
		if cfg.PositionSide == "LONG" && currentPrice < cfg.ActivationPrice {
			return
		}
		if cfg.PositionSide == "SHORT" && currentPrice > cfg.ActivationPrice {
			return
		}
	}

	highestPrice := cfg.HighestPrice
	lowestPrice := cfg.LowestPrice
	currentStop := cfg.CurrentStopPrice

	// Update extremes
	if cfg.PositionSide == "LONG" {
		if currentPrice > highestPrice || highestPrice == 0 {
			highestPrice = currentPrice
		}
	} else {
		if currentPrice < lowestPrice || lowestPrice == 0 {
			lowestPrice = currentPrice
		}
	}

	// Calculate trailing distance
	var trailingDist float64
	if cfg.TrailingMode == "percent" {
		if cfg.PositionSide == "LONG" {
			trailingDist = highestPrice * cfg.TrailingDistance / 100.0
		} else {
			trailingDist = lowestPrice * cfg.TrailingDistance / 100.0
		}
	} else {
		trailingDist = cfg.TrailingDistance
	}

	// Calculate new stop price
	var newStop float64
	if cfg.PositionSide == "LONG" {
		newStop = highestPrice - trailingDist
		if cfg.StopType == "stop_loss" {
			if currentStop == 0 || newStop > currentStop {
				currentStop = newStop
			}
		} else {
			currentStop = newStop
		}
	} else {
		newStop = lowestPrice + trailingDist
		if cfg.StopType == "stop_loss" {
			if currentStop == 0 || newStop < currentStop {
				currentStop = newStop
			}
		} else {
			currentStop = newStop
		}
	}

	// Persist updates
	t.store.TrailingStop().UpdateTrailingStopPrice(cfg.ID, currentStop, highestPrice, lowestPrice)

	// Check trigger
	triggered := false
	if cfg.PositionSide == "LONG" {
		triggered = currentPrice <= currentStop
	} else {
		triggered = currentPrice >= currentStop
	}

	if triggered {
		t.triggerStop(cfg, currentPrice)
	}
}

func (t *TrailingStopTracker) triggerStop(cfg *store.TrailingStopConfigModel, price float64) {
	t.tradersMu.RLock()
	at, ok := t.traders[cfg.TraderID]
	t.tradersMu.RUnlock()
	if !ok {
		logger.Warnf("[TrailingStop] Trader %s not registered, marking config %s as triggered", cfg.TraderID, cfg.ID)
		t.store.TrailingStop().TriggerTrailingStop(cfg.ID, price)
		return
	}

	trader := at.GetUnderlyingTrader()
	var execErr error

	if cfg.PositionSide == "LONG" {
		_, execErr = trader.CloseLong(cfg.Symbol, cfg.Quantity)
	} else {
		_, execErr = trader.CloseShort(cfg.Symbol, cfg.Quantity)
	}

	if execErr != nil {
		logger.Errorf("[TrailingStop] Failed to close position for %s: %v", cfg.ID, execErr)
		return
	}

	t.store.TrailingStop().TriggerTrailingStop(cfg.ID, price)
	logger.Infof("[TrailingStop] %s triggered for %s %s @ %.4f (stop=%.4f)",
		cfg.StopType, cfg.Symbol, cfg.PositionSide, price, cfg.CurrentStopPrice)
}
