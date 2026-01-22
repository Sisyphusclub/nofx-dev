//go:build cgo

package trader

import (
	"errors"
	"fmt"
	"math"
	"nofx/store"
	"sync"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupTrailingStopTestStore(t *testing.T) *store.Store {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	st, err := store.NewFromGorm(db)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	if err := st.GormDB().AutoMigrate(&store.TrailingStopConfigModel{}); err != nil {
		t.Fatalf("Failed to init trailing stop tables: %v", err)
	}
	return st
}

func assertFloatEqual(t *testing.T, name string, got, want float64) {
	t.Helper()
	const eps = 1e-6
	if math.Abs(got-want) > eps {
		t.Errorf("Expected %s %.6f, got %.6f", name, want, got)
	}
}

func TestTrailingStopTracker_RegisterUnregister(t *testing.T) {
	st := setupTrailingStopTestStore(t)
	tracker := NewTrailingStopTracker(st)

	tracker.RegisterTrader("trader-1", nil)
	tracker.RegisterTrader("trader-2", nil)

	tracker.tradersMu.RLock()
	count := len(tracker.traders)
	tracker.tradersMu.RUnlock()

	if count != 2 {
		t.Errorf("Expected 2 registered traders, got %d", count)
	}

	tracker.UnregisterTrader("trader-1")

	tracker.tradersMu.RLock()
	count = len(tracker.traders)
	tracker.tradersMu.RUnlock()

	if count != 1 {
		t.Errorf("Expected 1 registered trader after unregister, got %d", count)
	}
}

func TestTrailingStopTracker_StartStop(t *testing.T) {
	st := setupTrailingStopTestStore(t)
	tracker := NewTrailingStopTracker(st)

	tracker.Start()
	time.Sleep(100 * time.Millisecond)
	tracker.Stop()
}

func TestTrailingStopTracker_CheckInterval(t *testing.T) {
	st := setupTrailingStopTestStore(t)
	tracker := NewTrailingStopTracker(st)

	if tracker.checkInterval != 1*time.Second {
		t.Errorf("Expected check interval 1s, got %v", tracker.checkInterval)
	}
}

func TestTrailingStopTracker_PercentCalculation(t *testing.T) {
	testCases := []struct {
		name          string
		positionSide  string
		highestPrice  float64
		lowestPrice   float64
		trailingDist  float64
		expectedStop  float64
	}{
		{
			name:          "LONG 2% trailing",
			positionSide:  "LONG",
			highestPrice:  50000,
			lowestPrice:   0,
			trailingDist:  2.0,
			expectedStop:  49000,
		},
		{
			name:          "SHORT 2% trailing",
			positionSide:  "SHORT",
			highestPrice:  0,
			lowestPrice:   50000,
			trailingDist:  2.0,
			expectedStop:  51000,
		},
		{
			name:          "LONG 5% trailing",
			positionSide:  "LONG",
			highestPrice:  100000,
			lowestPrice:   0,
			trailingDist:  5.0,
			expectedStop:  95000,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var trailingDist float64
			var newStop float64

			if tc.positionSide == "LONG" {
				trailingDist = tc.highestPrice * tc.trailingDist / 100.0
				newStop = tc.highestPrice - trailingDist
			} else {
				trailingDist = tc.lowestPrice * tc.trailingDist / 100.0
				newStop = tc.lowestPrice + trailingDist
			}

			if newStop != tc.expectedStop {
				t.Errorf("Expected stop %.2f, got %.2f", tc.expectedStop, newStop)
			}
		})
	}
}

func TestTrailingStopTracker_FixedPointsCalculation(t *testing.T) {
	testCases := []struct {
		name          string
		positionSide  string
		highestPrice  float64
		lowestPrice   float64
		trailingDist  float64
		expectedStop  float64
	}{
		{
			name:          "LONG fixed 500 points",
			positionSide:  "LONG",
			highestPrice:  50000,
			lowestPrice:   0,
			trailingDist:  500,
			expectedStop:  49500,
		},
		{
			name:          "SHORT fixed 500 points",
			positionSide:  "SHORT",
			highestPrice:  0,
			lowestPrice:   50000,
			trailingDist:  500,
			expectedStop:  50500,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var newStop float64

			if tc.positionSide == "LONG" {
				newStop = tc.highestPrice - tc.trailingDist
			} else {
				newStop = tc.lowestPrice + tc.trailingDist
			}

			if newStop != tc.expectedStop {
				t.Errorf("Expected stop %.2f, got %.2f", tc.expectedStop, newStop)
			}
		})
	}
}

func TestTrailingStopTracker_TriggerConditions(t *testing.T) {
	testCases := []struct {
		name         string
		positionSide string
		currentPrice float64
		currentStop  float64
		shouldTrig   bool
	}{
		{"LONG triggered", "LONG", 49000, 49500, true},
		{"LONG exact", "LONG", 49500, 49500, true},
		{"LONG not triggered", "LONG", 50000, 49500, false},
		{"SHORT triggered", "SHORT", 51000, 50500, true},
		{"SHORT exact", "SHORT", 50500, 50500, true},
		{"SHORT not triggered", "SHORT", 50000, 50500, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var triggered bool
			if tc.positionSide == "LONG" {
				triggered = tc.currentPrice <= tc.currentStop
			} else {
				triggered = tc.currentPrice >= tc.currentStop
			}
			if triggered != tc.shouldTrig {
				t.Errorf("Expected triggered=%v for %s, price=%.0f, stop=%.0f",
					tc.shouldTrig, tc.positionSide, tc.currentPrice, tc.currentStop)
			}
		})
	}
}

func TestTrailingStopTracker_ActivationPrice(t *testing.T) {
	testCases := []struct {
		name            string
		positionSide    string
		activationPrice float64
		currentPrice    float64
		shouldActivate  bool
	}{
		{"LONG not activated", "LONG", 52000, 51000, false},
		{"LONG activated", "LONG", 52000, 53000, true},
		{"LONG exact activation", "LONG", 52000, 52000, true},
		{"SHORT not activated", "SHORT", 48000, 49000, false},
		{"SHORT activated", "SHORT", 48000, 47000, true},
		{"SHORT exact activation", "SHORT", 48000, 48000, true},
		{"No activation price LONG", "LONG", 0, 50000, true},
		{"No activation price SHORT", "SHORT", 0, 50000, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			shouldActivate := true

			if tc.activationPrice > 0 {
				if tc.positionSide == "LONG" && tc.currentPrice < tc.activationPrice {
					shouldActivate = false
				}
				if tc.positionSide == "SHORT" && tc.currentPrice > tc.activationPrice {
					shouldActivate = false
				}
			}

			if shouldActivate != tc.shouldActivate {
				t.Errorf("Expected activation=%v for %s, activation=%.0f, current=%.0f",
					tc.shouldActivate, tc.positionSide, tc.activationPrice, tc.currentPrice)
			}
		})
	}
}

func TestTrailingStopTracker_StopLossOnlyMovesInFavor(t *testing.T) {
	currentStop := 49000.0

	newStop1 := 49500.0
	if newStop1 > currentStop {
		currentStop = newStop1
	}
	if currentStop != 49500 {
		t.Errorf("Stop should move up for LONG, expected 49500, got %.0f", currentStop)
	}

	newStop2 := 49000.0
	if newStop2 > currentStop {
		currentStop = newStop2
	}
	if currentStop != 49500 {
		t.Errorf("Stop should NOT move down for LONG, expected 49500, got %.0f", currentStop)
	}

	shortStop := 51000.0

	newShortStop1 := 50500.0
	if newShortStop1 < shortStop {
		shortStop = newShortStop1
	}
	if shortStop != 50500 {
		t.Errorf("Stop should move down for SHORT, expected 50500, got %.0f", shortStop)
	}

	newShortStop2 := 51000.0
	if newShortStop2 < shortStop {
		shortStop = newShortStop2
	}
	if shortStop != 50500 {
		t.Errorf("Stop should NOT move up for SHORT, expected 50500, got %.0f", shortStop)
	}
}

func TestTrailingStopTracker_ActivationGate(t *testing.T) {
	st := setupTrailingStopTestStore(t)
	tracker := NewTrailingStopTracker(st)

	cfg := &store.TrailingStopConfigModel{
		TraderID:         "trader-1",
		Symbol:           "BTCUSDT",
		PositionSide:     "LONG",
		StopType:         "stop_loss",
		TrailingMode:     "percent",
		TrailingDistance: 2.0,
		ActivationPrice:  52000.0,
		Quantity:         0.1,
		IsActive:         true,
		Status:           "active",
	}

	if err := st.TrailingStop().CreateTrailingStop(cfg); err != nil {
		t.Fatalf("CreateTrailingStop failed: %v", err)
	}

	tracker.processConfig(cfg, 51000)

	updated, err := st.TrailingStop().GetTrailingStop(cfg.ID)
	if err != nil {
		t.Fatalf("GetTrailingStop failed: %v", err)
	}

	assertFloatEqual(t, "highest price", updated.HighestPrice, 0)
	assertFloatEqual(t, "lowest price", updated.LowestPrice, 0)
	assertFloatEqual(t, "current stop", updated.CurrentStopPrice, 0)
	if updated.Status != "active" || !updated.IsActive {
		t.Errorf("Expected active config to remain active, got status=%s is_active=%v", updated.Status, updated.IsActive)
	}
}

func TestTrailingStopTracker_ProcessConfigLong(t *testing.T) {
	st := setupTrailingStopTestStore(t)
	tracker := NewTrailingStopTracker(st)

	cfg := &store.TrailingStopConfigModel{
		TraderID:         "trader-1",
		Symbol:           "BTCUSDT",
		PositionSide:     "LONG",
		StopType:         "stop_loss",
		TrailingMode:     "percent",
		TrailingDistance: 2.0,
		Quantity:         0.1,
		IsActive:         true,
		Status:           "active",
	}

	if err := st.TrailingStop().CreateTrailingStop(cfg); err != nil {
		t.Fatalf("CreateTrailingStop failed: %v", err)
	}

	tracker.processConfig(cfg, 50000)

	updated, err := st.TrailingStop().GetTrailingStop(cfg.ID)
	if err != nil {
		t.Fatalf("GetTrailingStop failed: %v", err)
	}
	assertFloatEqual(t, "highest price", updated.HighestPrice, 50000)
	assertFloatEqual(t, "current stop", updated.CurrentStopPrice, 49000)

	cfg, _ = st.TrailingStop().GetTrailingStop(cfg.ID)
	tracker.processConfig(cfg, 51000)

	updated, _ = st.TrailingStop().GetTrailingStop(cfg.ID)
	assertFloatEqual(t, "highest price", updated.HighestPrice, 51000)
	assertFloatEqual(t, "current stop", updated.CurrentStopPrice, 49980)

	cfg, _ = st.TrailingStop().GetTrailingStop(cfg.ID)
	tracker.processConfig(cfg, 50500)

	updated, _ = st.TrailingStop().GetTrailingStop(cfg.ID)
	assertFloatEqual(t, "highest price", updated.HighestPrice, 51000)
	assertFloatEqual(t, "current stop", updated.CurrentStopPrice, 49980)
}

func TestTrailingStopTracker_ProcessConfigShort(t *testing.T) {
	st := setupTrailingStopTestStore(t)
	tracker := NewTrailingStopTracker(st)

	cfg := &store.TrailingStopConfigModel{
		TraderID:         "trader-1",
		Symbol:           "ETHUSDT",
		PositionSide:     "SHORT",
		StopType:         "take_profit",
		TrailingMode:     "fixed_points",
		TrailingDistance: 100.0,
		Quantity:         1.0,
		IsActive:         true,
		Status:           "active",
	}

	if err := st.TrailingStop().CreateTrailingStop(cfg); err != nil {
		t.Fatalf("CreateTrailingStop failed: %v", err)
	}

	tracker.processConfig(cfg, 2000)

	updated, err := st.TrailingStop().GetTrailingStop(cfg.ID)
	if err != nil {
		t.Fatalf("GetTrailingStop failed: %v", err)
	}
	assertFloatEqual(t, "lowest price", updated.LowestPrice, 2000)
	assertFloatEqual(t, "current stop", updated.CurrentStopPrice, 2100)

	cfg, _ = st.TrailingStop().GetTrailingStop(cfg.ID)
	tracker.processConfig(cfg, 1900)

	updated, _ = st.TrailingStop().GetTrailingStop(cfg.ID)
	assertFloatEqual(t, "lowest price", updated.LowestPrice, 1900)
	assertFloatEqual(t, "current stop", updated.CurrentStopPrice, 2000)

	cfg, _ = st.TrailingStop().GetTrailingStop(cfg.ID)
	tracker.processConfig(cfg, 1950)

	updated, _ = st.TrailingStop().GetTrailingStop(cfg.ID)
	assertFloatEqual(t, "lowest price", updated.LowestPrice, 1900)
	assertFloatEqual(t, "current stop", updated.CurrentStopPrice, 2000)
}

func TestTrailingStopTracker_TriggerMarksConfig(t *testing.T) {
	st := setupTrailingStopTestStore(t)
	tracker := NewTrailingStopTracker(st)

	cfg := &store.TrailingStopConfigModel{
		TraderID:         "trader-1",
		Symbol:           "BTCUSDT",
		PositionSide:     "LONG",
		StopType:         "take_profit",
		TrailingMode:     "fixed_points",
		TrailingDistance: 500.0,
		Quantity:         0.1,
		CurrentStopPrice: 9500.0,
		HighestPrice:     10000.0,
		IsActive:         true,
		Status:           "active",
	}

	if err := st.TrailingStop().CreateTrailingStop(cfg); err != nil {
		t.Fatalf("CreateTrailingStop failed: %v", err)
	}

	tracker.processConfig(cfg, 9400)

	updated, err := st.TrailingStop().GetTrailingStop(cfg.ID)
	if err != nil {
		t.Fatalf("GetTrailingStop failed: %v", err)
	}
	if updated.Status != "triggered" || updated.IsActive {
		t.Errorf("Expected triggered config, got status=%s is_active=%v", updated.Status, updated.IsActive)
	}
	assertFloatEqual(t, "triggered price", updated.TriggeredPrice, 9400)
	if updated.TriggeredAt == nil {
		t.Error("Expected TriggeredAt to be set")
	}
}

func TestTrailingStopTracker_ConcurrentProcessing(t *testing.T) {
	st := setupTrailingStopTestStore(t)
	tracker := NewTrailingStopTracker(st)

	for i := 0; i < 5; i++ {
		cfg := &store.TrailingStopConfigModel{
			TraderID:         fmt.Sprintf("trader-%d", i),
			Symbol:           "BTCUSDT",
			PositionSide:     "LONG",
			StopType:         "stop_loss",
			TrailingMode:     "percent",
			TrailingDistance: 2.0,
			Quantity:         0.1,
			IsActive:         true,
			Status:           "active",
		}
		if err := st.TrailingStop().CreateTrailingStop(cfg); err != nil {
			t.Fatalf("CreateTrailingStop failed: %v", err)
		}
	}

	done := make(chan bool)
	for i := 0; i < 5; i++ {
		go func(traderNum int) {
			traderID := fmt.Sprintf("trader-%d", traderNum)
			tracker.RegisterTrader(traderID, nil)
			time.Sleep(10 * time.Millisecond)
			tracker.UnregisterTrader(traderID)
			done <- true
		}(i)
	}

	for i := 0; i < 5; i++ {
		<-done
	}

	tracker.tradersMu.RLock()
	count := len(tracker.traders)
	tracker.tradersMu.RUnlock()

	if count != 0 {
		t.Errorf("Expected 0 traders after concurrent ops, got %d", count)
	}
}

func TestTrailingStopTracker_MultipleConfigsProcessing(t *testing.T) {
	st := setupTrailingStopTestStore(t)
	tracker := NewTrailingStopTracker(st)

	longCfg := &store.TrailingStopConfigModel{
		TraderID:         "trader-1",
		Symbol:           "BTCUSDT",
		PositionSide:     "LONG",
		StopType:         "stop_loss",
		TrailingMode:     "percent",
		TrailingDistance: 2.0,
		Quantity:         0.1,
		IsActive:         true,
		Status:           "active",
	}
	shortCfg := &store.TrailingStopConfigModel{
		TraderID:         "trader-1",
		Symbol:           "ETHUSDT",
		PositionSide:     "SHORT",
		StopType:         "stop_loss",
		TrailingMode:     "fixed_points",
		TrailingDistance: 50.0,
		Quantity:         1.0,
		IsActive:         true,
		Status:           "active",
	}

	st.TrailingStop().CreateTrailingStop(longCfg)
	st.TrailingStop().CreateTrailingStop(shortCfg)

	tracker.processConfig(longCfg, 50000)
	longCfg, _ = st.TrailingStop().GetTrailingStop(longCfg.ID)
	assertFloatEqual(t, "long highest", longCfg.HighestPrice, 50000)
	assertFloatEqual(t, "long stop", longCfg.CurrentStopPrice, 49000)

	tracker.processConfig(shortCfg, 2000)
	shortCfg, _ = st.TrailingStop().GetTrailingStop(shortCfg.ID)
	assertFloatEqual(t, "short lowest", shortCfg.LowestPrice, 2000)
	assertFloatEqual(t, "short stop", shortCfg.CurrentStopPrice, 2050)
}

func TestTrailingStopTracker_CancelConfig(t *testing.T) {
	st := setupTrailingStopTestStore(t)

	cfg := &store.TrailingStopConfigModel{
		TraderID:         "trader-1",
		Symbol:           "BTCUSDT",
		PositionSide:     "LONG",
		StopType:         "stop_loss",
		TrailingMode:     "percent",
		TrailingDistance: 2.0,
		Quantity:         0.1,
		IsActive:         true,
		Status:           "active",
	}
	if err := st.TrailingStop().CreateTrailingStop(cfg); err != nil {
		t.Fatalf("CreateTrailingStop failed: %v", err)
	}

	if err := st.TrailingStop().CancelTrailingStop(cfg.ID); err != nil {
		t.Fatalf("CancelTrailingStop failed: %v", err)
	}

	updated, _ := st.TrailingStop().GetTrailingStop(cfg.ID)
	if updated.Status != "canceled" || updated.IsActive {
		t.Errorf("Expected canceled config, got status=%s is_active=%v", updated.Status, updated.IsActive)
	}
}

func TestTrailingStopTracker_GetActiveByTrader(t *testing.T) {
	st := setupTrailingStopTestStore(t)

	for i := 0; i < 3; i++ {
		cfg := &store.TrailingStopConfigModel{
			TraderID:         "trader-1",
			Symbol:           fmt.Sprintf("SYM%dUSDT", i),
			PositionSide:     "LONG",
			StopType:         "stop_loss",
			TrailingMode:     "percent",
			TrailingDistance: 2.0,
			Quantity:         0.1,
			IsActive:         true,
			Status:           "active",
		}
		st.TrailingStop().CreateTrailingStop(cfg)
	}

	canceledCfg := &store.TrailingStopConfigModel{
		TraderID:         "trader-1",
		Symbol:           "CANCELUSDT",
		PositionSide:     "LONG",
		StopType:         "stop_loss",
		TrailingMode:     "percent",
		TrailingDistance: 2.0,
		Quantity:         0.1,
		IsActive:         false,
		Status:           "canceled",
	}
	st.TrailingStop().CreateTrailingStop(canceledCfg)

	active, err := st.TrailingStop().GetActiveTrailingStops("trader-1")
	if err != nil {
		t.Fatalf("GetActiveTrailingStops failed: %v", err)
	}
	if len(active) != 3 {
		t.Errorf("Expected 3 active configs, got %d", len(active))
	}
}

func TestTrailingStopTracker_TakeProfitMovesBothDirections(t *testing.T) {
	st := setupTrailingStopTestStore(t)
	tracker := NewTrailingStopTracker(st)

	cfg := &store.TrailingStopConfigModel{
		TraderID:         "trader-1",
		Symbol:           "BTCUSDT",
		PositionSide:     "LONG",
		StopType:         "take_profit",
		TrailingMode:     "fixed_points",
		TrailingDistance: 500.0,
		Quantity:         0.1,
		IsActive:         true,
		Status:           "active",
	}
	st.TrailingStop().CreateTrailingStop(cfg)

	tracker.processConfig(cfg, 50000)
	cfg, _ = st.TrailingStop().GetTrailingStop(cfg.ID)
	assertFloatEqual(t, "stop after 50000", cfg.CurrentStopPrice, 49500)

	tracker.processConfig(cfg, 51000)
	cfg, _ = st.TrailingStop().GetTrailingStop(cfg.ID)
	assertFloatEqual(t, "stop after 51000", cfg.CurrentStopPrice, 50500)

	tracker.processConfig(cfg, 50500)
	cfg, _ = st.TrailingStop().GetTrailingStop(cfg.ID)
	assertFloatEqual(t, "stop after pullback", cfg.CurrentStopPrice, 50000)
}

func TestTrailingStopTracker_ConcurrentRegisterUnregister(t *testing.T) {
	st := setupTrailingStopTestStore(t)
	tracker := NewTrailingStopTracker(st)

	const workers = 20
	const loops = 100

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		traderID := fmt.Sprintf("trader-%d", i)
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			for j := 0; j < loops; j++ {
				tracker.RegisterTrader(id, nil)
				tracker.UnregisterTrader(id)
			}
		}(traderID)
	}
	wg.Wait()

	for i := 0; i < workers; i++ {
		tracker.UnregisterTrader(fmt.Sprintf("trader-%d", i))
	}

	tracker.tradersMu.RLock()
	count := len(tracker.traders)
	tracker.tradersMu.RUnlock()

	if count != 0 {
		t.Errorf("Expected 0 registered traders, got %d", count)
	}
}

func TestTrailingStopTracker_ProcessMultipleConfigs(t *testing.T) {
	st := setupTrailingStopTestStore(t)
	tracker := NewTrailingStopTracker(st)

	testCases := []struct {
		name     string
		cfg      store.TrailingStopConfigModel
		price    float64
		wantStop float64
		wantHigh float64
		wantLow  float64
	}{
		{
			name: "LONG percent trailing",
			cfg: store.TrailingStopConfigModel{
				TraderID:         "trader-1",
				Symbol:           "BTCUSDT",
				PositionSide:     "LONG",
				StopType:         "stop_loss",
				TrailingMode:     "percent",
				TrailingDistance: 2.0,
				Quantity:         0.1,
				IsActive:         true,
				Status:           "active",
			},
			price:    50000,
			wantStop: 49000,
			wantHigh: 50000,
			wantLow:  0,
		},
		{
			name: "SHORT fixed points trailing",
			cfg: store.TrailingStopConfigModel{
				TraderID:         "trader-2",
				Symbol:           "ETHUSDT",
				PositionSide:     "SHORT",
				StopType:         "take_profit",
				TrailingMode:     "fixed_points",
				TrailingDistance: 100.0,
				Quantity:         1.0,
				IsActive:         true,
				Status:           "active",
			},
			price:    2000,
			wantStop: 2100,
			wantHigh: 0,
			wantLow:  2000,
		},
		{
			name: "LONG fixed points take profit",
			cfg: store.TrailingStopConfigModel{
				TraderID:         "trader-3",
				Symbol:           "XRPUSDT",
				PositionSide:     "LONG",
				StopType:         "take_profit",
				TrailingMode:     "fixed_points",
				TrailingDistance: 0.1,
				Quantity:         100,
				IsActive:         true,
				Status:           "active",
			},
			price:    1.5,
			wantStop: 1.4,
			wantHigh: 1.5,
			wantLow:  0,
		},
	}

	configs := make([]*store.TrailingStopConfigModel, 0, len(testCases))
	for _, tc := range testCases {
		cfg := tc.cfg
		if err := st.TrailingStop().CreateTrailingStop(&cfg); err != nil {
			t.Fatalf("CreateTrailingStop failed: %v", err)
		}
		configs = append(configs, &cfg)
	}

	for i, tc := range testCases {
		tracker.processConfig(configs[i], tc.price)

		updated, err := st.TrailingStop().GetTrailingStop(configs[i].ID)
		if err != nil {
			t.Fatalf("GetTrailingStop failed: %v", err)
		}
		assertFloatEqual(t, "current stop", updated.CurrentStopPrice, tc.wantStop)
		assertFloatEqual(t, "highest price", updated.HighestPrice, tc.wantHigh)
		assertFloatEqual(t, "lowest price", updated.LowestPrice, tc.wantLow)
	}
}

func TestTrailingStopTracker_ErrorRecoveryOnCloseFailure(t *testing.T) {
	st := setupTrailingStopTestStore(t)
	tracker := NewTrailingStopTracker(st)

	mock := &mockTrader{
		closeLongErr: errors.New("close failed"),
	}
	tracker.RegisterTrader("trader-1", &AutoTrader{trader: mock})

	cfg := &store.TrailingStopConfigModel{
		TraderID:         "trader-1",
		Symbol:           "BTCUSDT",
		PositionSide:     "LONG",
		StopType:         "stop_loss",
		TrailingMode:     "fixed_points",
		TrailingDistance: 1.0,
		Quantity:         0.1,
		CurrentStopPrice: 99.0,
		HighestPrice:     100.0,
		IsActive:         true,
		Status:           "active",
	}

	if err := st.TrailingStop().CreateTrailingStop(cfg); err != nil {
		t.Fatalf("CreateTrailingStop failed: %v", err)
	}

	tracker.processConfig(cfg, 98.0)

	updated, err := st.TrailingStop().GetTrailingStop(cfg.ID)
	if err != nil {
		t.Fatalf("GetTrailingStop failed: %v", err)
	}
	if updated.Status != "active" || !updated.IsActive {
		t.Errorf("Expected active config after close error, got status=%s is_active=%v", updated.Status, updated.IsActive)
	}
	if updated.TriggeredAt != nil {
		t.Error("TriggeredAt should not be set on close error")
	}

	openLong, openShort, closeLong, closeShort := mock.calls()
	if closeLong != 1 || openLong != 0 || openShort != 0 || closeShort != 0 {
		t.Errorf("Unexpected call counts: openLong=%d openShort=%d closeLong=%d closeShort=%d",
			openLong, openShort, closeLong, closeShort)
	}
}
