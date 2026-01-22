//go:build cgo

package store

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupTestLimitOrderDB(t *testing.T) *LimitOrderStore {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	store := NewLimitOrderStore(db)
	if err := store.initTables(); err != nil {
		t.Fatalf("Failed to init tables: %v", err)
	}
	return store
}

func TestLimitOrderStore_CreateAndGet(t *testing.T) {
	store := setupTestLimitOrderDB(t)

	order := &LimitOrderModel{
		TraderID:         "trader-1",
		ExchangeID:       "binance",
		Symbol:           "BTCUSDT",
		Side:             "BUY",
		PositionSide:     "LONG",
		TriggerCondition: "lte",
		TriggerPrice:     50000.0,
		Quantity:         0.1,
		Leverage:         10,
		Status:           "pending",
	}

	if err := store.CreateLimitOrder(order); err != nil {
		t.Fatalf("CreateLimitOrder failed: %v", err)
	}

	if order.ID == "" {
		t.Error("Order ID should be generated")
	}

	retrieved, err := store.GetLimitOrder(order.ID)
	if err != nil {
		t.Fatalf("GetLimitOrder failed: %v", err)
	}

	if retrieved.Symbol != "BTCUSDT" {
		t.Errorf("Expected symbol BTCUSDT, got %s", retrieved.Symbol)
	}
	if retrieved.TriggerPrice != 50000.0 {
		t.Errorf("Expected trigger price 50000, got %.2f", retrieved.TriggerPrice)
	}
	if retrieved.Status != "pending" {
		t.Errorf("Expected status pending, got %s", retrieved.Status)
	}
}

func TestLimitOrderStore_GetPendingOrders(t *testing.T) {
	store := setupTestLimitOrderDB(t)

	orders := []*LimitOrderModel{
		{TraderID: "trader-1", Symbol: "BTCUSDT", Side: "BUY", PositionSide: "LONG", TriggerCondition: "lte", TriggerPrice: 50000, Quantity: 0.1, Status: "pending"},
		{TraderID: "trader-1", Symbol: "ETHUSDT", Side: "SELL", PositionSide: "SHORT", TriggerCondition: "gte", TriggerPrice: 3000, Quantity: 1.0, Status: "pending"},
		{TraderID: "trader-1", Symbol: "BTCUSDT", Side: "SELL", PositionSide: "LONG", TriggerCondition: "gte", TriggerPrice: 55000, Quantity: 0.1, Status: "filled"},
		{TraderID: "trader-2", Symbol: "BTCUSDT", Side: "BUY", PositionSide: "LONG", TriggerCondition: "lte", TriggerPrice: 49000, Quantity: 0.2, Status: "pending"},
	}

	for _, o := range orders {
		if err := store.CreateLimitOrder(o); err != nil {
			t.Fatalf("CreateLimitOrder failed: %v", err)
		}
	}

	pending, err := store.GetPendingLimitOrders("trader-1")
	if err != nil {
		t.Fatalf("GetPendingLimitOrders failed: %v", err)
	}
	if len(pending) != 2 {
		t.Errorf("Expected 2 pending orders for trader-1, got %d", len(pending))
	}

	allPending, err := store.GetAllPendingOrders()
	if err != nil {
		t.Fatalf("GetAllPendingOrders failed: %v", err)
	}
	if len(allPending) != 3 {
		t.Errorf("Expected 3 total pending orders, got %d", len(allPending))
	}
}

func TestLimitOrderStore_UpdateStatus(t *testing.T) {
	store := setupTestLimitOrderDB(t)

	order := &LimitOrderModel{
		TraderID:         "trader-1",
		Symbol:           "BTCUSDT",
		Side:             "BUY",
		PositionSide:     "LONG",
		TriggerCondition: "lte",
		TriggerPrice:     50000,
		Quantity:         0.1,
		Status:           "pending",
	}

	store.CreateLimitOrder(order)

	err := store.UpdateLimitOrderStatus(order.ID, "filled", 49500.0, 0.1, 0.001)
	if err != nil {
		t.Fatalf("UpdateLimitOrderStatus failed: %v", err)
	}

	updated, _ := store.GetLimitOrder(order.ID)
	if updated.Status != "filled" {
		t.Errorf("Expected status filled, got %s", updated.Status)
	}
	if updated.FilledPrice != 49500.0 {
		t.Errorf("Expected filled price 49500, got %.2f", updated.FilledPrice)
	}
	if updated.Commission != 0.001 {
		t.Errorf("Expected commission 0.001, got %.4f", updated.Commission)
	}
}

func TestLimitOrderStore_Cancel(t *testing.T) {
	store := setupTestLimitOrderDB(t)

	order := &LimitOrderModel{
		TraderID:         "trader-1",
		Symbol:           "BTCUSDT",
		Side:             "BUY",
		PositionSide:     "LONG",
		TriggerCondition: "lte",
		TriggerPrice:     50000,
		Quantity:         0.1,
		Status:           "pending",
	}

	store.CreateLimitOrder(order)

	if err := store.CancelLimitOrder(order.ID); err != nil {
		t.Fatalf("CancelLimitOrder failed: %v", err)
	}

	canceled, _ := store.GetLimitOrder(order.ID)
	if canceled.Status != "canceled" {
		t.Errorf("Expected status canceled, got %s", canceled.Status)
	}
}

func TestLimitOrderStore_ExpireOrders(t *testing.T) {
	store := setupTestLimitOrderDB(t)

	pastTime := time.Now().UTC().Add(-1 * time.Hour)
	futureTime := time.Now().UTC().Add(1 * time.Hour)

	orders := []*LimitOrderModel{
		{TraderID: "trader-1", Symbol: "BTCUSDT", Side: "BUY", PositionSide: "LONG", TriggerCondition: "lte", TriggerPrice: 50000, Quantity: 0.1, Status: "pending", ExpiresAt: &pastTime},
		{TraderID: "trader-1", Symbol: "ETHUSDT", Side: "BUY", PositionSide: "LONG", TriggerCondition: "lte", TriggerPrice: 3000, Quantity: 1.0, Status: "pending", ExpiresAt: &futureTime},
		{TraderID: "trader-1", Symbol: "XRPUSDT", Side: "BUY", PositionSide: "LONG", TriggerCondition: "lte", TriggerPrice: 1, Quantity: 100, Status: "pending"},
	}

	for _, o := range orders {
		store.CreateLimitOrder(o)
	}

	expired, err := store.ExpireLimitOrders()
	if err != nil {
		t.Fatalf("ExpireLimitOrders failed: %v", err)
	}
	if expired != 1 {
		t.Errorf("Expected 1 expired order, got %d", expired)
	}

	order1, _ := store.GetLimitOrder(orders[0].ID)
	if order1.Status != "expired" {
		t.Errorf("Order 1 should be expired, got %s", order1.Status)
	}

	order2, _ := store.GetLimitOrder(orders[1].ID)
	if order2.Status != "pending" {
		t.Errorf("Order 2 should still be pending, got %s", order2.Status)
	}

	order3, _ := store.GetLimitOrder(orders[2].ID)
	if order3.Status != "pending" {
		t.Errorf("Order 3 should still be pending (no expiry), got %s", order3.Status)
	}
}

func TestLimitOrderStore_GetOrdersWithFilters(t *testing.T) {
	store := setupTestLimitOrderDB(t)

	orders := []*LimitOrderModel{
		{TraderID: "trader-1", Symbol: "BTCUSDT", Side: "BUY", PositionSide: "LONG", TriggerCondition: "lte", TriggerPrice: 50000, Quantity: 0.1, Status: "pending"},
		{TraderID: "trader-1", Symbol: "BTCUSDT", Side: "SELL", PositionSide: "LONG", TriggerCondition: "gte", TriggerPrice: 55000, Quantity: 0.1, Status: "filled"},
		{TraderID: "trader-1", Symbol: "ETHUSDT", Side: "BUY", PositionSide: "LONG", TriggerCondition: "lte", TriggerPrice: 3000, Quantity: 1.0, Status: "pending"},
	}

	for _, o := range orders {
		store.CreateLimitOrder(o)
	}

	btcOrders, err := store.GetLimitOrders("trader-1", "BTCUSDT", "")
	if err != nil {
		t.Fatalf("GetLimitOrders failed: %v", err)
	}
	if len(btcOrders) != 2 {
		t.Errorf("Expected 2 BTC orders, got %d", len(btcOrders))
	}

	pendingOrders, err := store.GetLimitOrders("trader-1", "", "pending")
	if err != nil {
		t.Fatalf("GetLimitOrders failed: %v", err)
	}
	if len(pendingOrders) != 2 {
		t.Errorf("Expected 2 pending orders, got %d", len(pendingOrders))
	}

	btcPending, err := store.GetLimitOrders("trader-1", "BTCUSDT", "pending")
	if err != nil {
		t.Fatalf("GetLimitOrders failed: %v", err)
	}
	if len(btcPending) != 1 {
		t.Errorf("Expected 1 pending BTC order, got %d", len(btcPending))
	}
}
