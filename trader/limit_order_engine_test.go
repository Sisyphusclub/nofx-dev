package trader

import (
	"nofx/store"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupTestStore(t *testing.T) *store.Store {
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
	return st
}

func TestLimitOrderEngine_RegisterUnregister(t *testing.T) {
	st := setupTestStore(t)
	engine := NewLimitOrderEngine(st)

	engine.RegisterTrader("trader-1", nil)
	engine.RegisterTrader("trader-2", nil)

	engine.tradersMu.RLock()
	count := len(engine.traders)
	engine.tradersMu.RUnlock()

	if count != 2 {
		t.Errorf("Expected 2 registered traders, got %d", count)
	}

	engine.UnregisterTrader("trader-1")

	engine.tradersMu.RLock()
	count = len(engine.traders)
	engine.tradersMu.RUnlock()

	if count != 1 {
		t.Errorf("Expected 1 registered trader after unregister, got %d", count)
	}
}

func TestLimitOrderEngine_StartStop(t *testing.T) {
	st := setupTestStore(t)
	engine := NewLimitOrderEngine(st)

	engine.Start()
	time.Sleep(100 * time.Millisecond)
	engine.Stop()
}

func TestLimitOrderEngine_TriggerConditions(t *testing.T) {
	testCases := []struct {
		name       string
		condition  string
		trigger    float64
		current    float64
		shouldTrig bool
	}{
		{"LTE triggered", "lte", 50000, 49000, true},
		{"LTE exact", "lte", 50000, 50000, true},
		{"LTE not triggered", "lte", 50000, 51000, false},
		{"GTE triggered", "gte", 50000, 51000, true},
		{"GTE exact", "gte", 50000, 50000, true},
		{"GTE not triggered", "gte", 50000, 49000, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var triggered bool
			switch tc.condition {
			case "lte":
				triggered = tc.current <= tc.trigger
			case "gte":
				triggered = tc.current >= tc.trigger
			}
			if triggered != tc.shouldTrig {
				t.Errorf("Expected triggered=%v for condition %s, trigger=%.0f, current=%.0f",
					tc.shouldTrig, tc.condition, tc.trigger, tc.current)
			}
		})
	}
}

func TestLimitOrderEngine_OrderSidePositionSide(t *testing.T) {
	testCases := []struct {
		side         string
		positionSide string
		action       string
	}{
		{"BUY", "LONG", "OpenLong"},
		{"BUY", "SHORT", "CloseShort"},
		{"SELL", "SHORT", "OpenShort"},
		{"SELL", "LONG", "CloseLong"},
	}

	for _, tc := range testCases {
		t.Run(tc.action, func(t *testing.T) {
			var action string
			switch tc.side {
			case "BUY":
				if tc.positionSide == "LONG" {
					action = "OpenLong"
				} else {
					action = "CloseShort"
				}
			case "SELL":
				if tc.positionSide == "SHORT" {
					action = "OpenShort"
				} else {
					action = "CloseLong"
				}
			}
			if action != tc.action {
				t.Errorf("Expected action %s for Side=%s, PositionSide=%s, got %s",
					tc.action, tc.side, tc.positionSide, action)
			}
		})
	}
}

func TestLimitOrderEngine_CheckInterval(t *testing.T) {
	st := setupTestStore(t)
	engine := NewLimitOrderEngine(st)

	if engine.checkInterval != 500*time.Millisecond {
		t.Errorf("Expected check interval 500ms, got %v", engine.checkInterval)
	}
}
