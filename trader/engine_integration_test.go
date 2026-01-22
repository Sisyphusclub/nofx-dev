//go:build cgo

package trader

import (
	"nofx/store"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupOrderEngineIntegrationStore(t *testing.T) *store.Store {
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
	if err := st.GormDB().AutoMigrate(&store.LimitOrderModel{}, &store.TrailingStopConfigModel{}); err != nil {
		t.Fatalf("Failed to init order engine tables: %v", err)
	}
	return st
}

func TestOrderEngines_IntegrationLimitOrderAndTrailingStop(t *testing.T) {
	st := setupOrderEngineIntegrationStore(t)
	engine := NewLimitOrderEngine(st)
	tracker := NewTrailingStopTracker(st)

	mock := &mockTrader{
		openLongResult: map[string]interface{}{
			"avgPrice":    50000.0,
			"executedQty": 0.1,
		},
	}
	at := &AutoTrader{trader: mock}

	engine.RegisterTrader("trader-1", at)
	tracker.RegisterTrader("trader-1", at)

	order := &store.LimitOrderModel{
		TraderID:         "trader-1",
		Symbol:           "BTCUSDT",
		Side:             "BUY",
		PositionSide:     "LONG",
		TriggerCondition: "lte",
		TriggerPrice:     50000,
		Quantity:         0.1,
		Leverage:         2,
		Status:           "pending",
	}
	if err := st.LimitOrder().CreateLimitOrder(order); err != nil {
		t.Fatalf("CreateLimitOrder failed: %v", err)
	}

	engine.executeOrder(order, 50000)

	updatedOrder, err := st.LimitOrder().GetLimitOrder(order.ID)
	if err != nil {
		t.Fatalf("GetLimitOrder failed: %v", err)
	}
	if updatedOrder.Status != "filled" {
		t.Errorf("Expected order status filled, got %s", updatedOrder.Status)
	}

	cfg := &store.TrailingStopConfigModel{
		TraderID:         "trader-1",
		Symbol:           "BTCUSDT",
		PositionSide:     "LONG",
		StopType:         "stop_loss",
		TrailingMode:     "fixed_points",
		TrailingDistance: 50.0,
		Quantity:         0.1,
		CurrentStopPrice: 49950.0,
		HighestPrice:     50000.0,
		IsActive:         true,
		Status:           "active",
	}

	if err := st.TrailingStop().CreateTrailingStop(cfg); err != nil {
		t.Fatalf("CreateTrailingStop failed: %v", err)
	}

	tracker.processConfig(cfg, 49900)

	updatedCfg, err := st.TrailingStop().GetTrailingStop(cfg.ID)
	if err != nil {
		t.Fatalf("GetTrailingStop failed: %v", err)
	}
	if updatedCfg.Status != "triggered" || updatedCfg.IsActive {
		t.Errorf("Expected triggered config, got status=%s is_active=%v", updatedCfg.Status, updatedCfg.IsActive)
	}

	openLong, openShort, closeLong, closeShort := mock.calls()
	if openLong != 1 || closeLong != 1 || openShort != 0 || closeShort != 0 {
		t.Errorf("Unexpected call counts: openLong=%d openShort=%d closeLong=%d closeShort=%d",
			openLong, openShort, closeLong, closeShort)
	}
}
