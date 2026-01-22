//go:build cgo

package trader

import (
	"errors"
	"fmt"
	"nofx/store"
	"sync"
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

func TestLimitOrderEngine_ExpireOrders(t *testing.T) {
	st := setupTestStore(t)
	if err := st.GormDB().AutoMigrate(&store.LimitOrderModel{}); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	pastTime := time.Now().UTC().Add(-1 * time.Hour)
	futureTime := time.Now().UTC().Add(1 * time.Hour)

	expiredOrder := &store.LimitOrderModel{
		TraderID:         "trader-1",
		Symbol:           "BTCUSDT",
		Side:             "BUY",
		PositionSide:     "LONG",
		TriggerCondition: "lte",
		TriggerPrice:     50000,
		Quantity:         0.1,
		Status:           "pending",
		ExpiresAt:        &pastTime,
	}
	activeOrder := &store.LimitOrderModel{
		TraderID:         "trader-1",
		Symbol:           "ETHUSDT",
		Side:             "SELL",
		PositionSide:     "SHORT",
		TriggerCondition: "gte",
		TriggerPrice:     2000,
		Quantity:         1.0,
		Status:           "pending",
		ExpiresAt:        &futureTime,
	}
	noExpiryOrder := &store.LimitOrderModel{
		TraderID:         "trader-1",
		Symbol:           "SOLUSDT",
		Side:             "BUY",
		PositionSide:     "LONG",
		TriggerCondition: "lte",
		TriggerPrice:     100,
		Quantity:         10.0,
		Status:           "pending",
	}

	if err := st.LimitOrder().CreateLimitOrder(expiredOrder); err != nil {
		t.Fatalf("Failed to create expired order: %v", err)
	}
	if err := st.LimitOrder().CreateLimitOrder(activeOrder); err != nil {
		t.Fatalf("Failed to create active order: %v", err)
	}
	if err := st.LimitOrder().CreateLimitOrder(noExpiryOrder); err != nil {
		t.Fatalf("Failed to create no-expiry order: %v", err)
	}

	count, err := st.LimitOrder().ExpireLimitOrders()
	if err != nil {
		t.Fatalf("ExpireLimitOrders failed: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 expired order, got %d", count)
	}

	updated, err := st.LimitOrder().GetLimitOrder(expiredOrder.ID)
	if err != nil {
		t.Fatalf("GetLimitOrder failed: %v", err)
	}
	if updated.Status != "expired" {
		t.Errorf("Expected status 'expired', got '%s'", updated.Status)
	}

	active, _ := st.LimitOrder().GetLimitOrder(activeOrder.ID)
	if active.Status != "pending" {
		t.Errorf("Active order should remain pending, got '%s'", active.Status)
	}

	noExpiry, _ := st.LimitOrder().GetLimitOrder(noExpiryOrder.ID)
	if noExpiry.Status != "pending" {
		t.Errorf("No-expiry order should remain pending, got '%s'", noExpiry.Status)
	}
}

func TestLimitOrderEngine_CancelOrder(t *testing.T) {
	st := setupTestStore(t)
	if err := st.GormDB().AutoMigrate(&store.LimitOrderModel{}); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	order := &store.LimitOrderModel{
		TraderID:         "trader-1",
		Symbol:           "BTCUSDT",
		Side:             "BUY",
		PositionSide:     "LONG",
		TriggerCondition: "lte",
		TriggerPrice:     50000,
		Quantity:         0.1,
		Status:           "pending",
	}
	if err := st.LimitOrder().CreateLimitOrder(order); err != nil {
		t.Fatalf("Failed to create order: %v", err)
	}

	if err := st.LimitOrder().CancelLimitOrder(order.ID); err != nil {
		t.Fatalf("CancelLimitOrder failed: %v", err)
	}

	updated, err := st.LimitOrder().GetLimitOrder(order.ID)
	if err != nil {
		t.Fatalf("GetLimitOrder failed: %v", err)
	}
	if updated.Status != "canceled" {
		t.Errorf("Expected status 'canceled', got '%s'", updated.Status)
	}
}

func TestLimitOrderEngine_GetPendingOrders(t *testing.T) {
	st := setupTestStore(t)
	if err := st.GormDB().AutoMigrate(&store.LimitOrderModel{}); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	for i := 0; i < 3; i++ {
		order := &store.LimitOrderModel{
			TraderID:         "trader-1",
			Symbol:           "BTCUSDT",
			Side:             "BUY",
			PositionSide:     "LONG",
			TriggerCondition: "lte",
			TriggerPrice:     float64(50000 + i*100),
			Quantity:         0.1,
			Status:           "pending",
		}
		st.LimitOrder().CreateLimitOrder(order)
	}

	filledOrder := &store.LimitOrderModel{
		TraderID:         "trader-1",
		Symbol:           "ETHUSDT",
		Side:             "SELL",
		PositionSide:     "SHORT",
		TriggerCondition: "gte",
		TriggerPrice:     2000,
		Quantity:         1.0,
		Status:           "filled",
	}
	st.LimitOrder().CreateLimitOrder(filledOrder)

	pending, err := st.LimitOrder().GetAllPendingOrders()
	if err != nil {
		t.Fatalf("GetAllPendingOrders failed: %v", err)
	}
	if len(pending) != 3 {
		t.Errorf("Expected 3 pending orders, got %d", len(pending))
	}

	traderPending, err := st.LimitOrder().GetPendingLimitOrders("trader-1")
	if err != nil {
		t.Fatalf("GetPendingLimitOrders failed: %v", err)
	}
	if len(traderPending) != 3 {
		t.Errorf("Expected 3 pending orders for trader-1, got %d", len(traderPending))
	}
}

func TestLimitOrderEngine_UpdateOrderStatus(t *testing.T) {
	st := setupTestStore(t)
	if err := st.GormDB().AutoMigrate(&store.LimitOrderModel{}); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	order := &store.LimitOrderModel{
		TraderID:         "trader-1",
		Symbol:           "BTCUSDT",
		Side:             "BUY",
		PositionSide:     "LONG",
		TriggerCondition: "lte",
		TriggerPrice:     50000,
		Quantity:         0.1,
		Status:           "pending",
	}
	if err := st.LimitOrder().CreateLimitOrder(order); err != nil {
		t.Fatalf("Failed to create order: %v", err)
	}

	err := st.LimitOrder().UpdateLimitOrderStatus(order.ID, "filled", 49500.50, 0.1, 0.005)
	if err != nil {
		t.Fatalf("UpdateLimitOrderStatus failed: %v", err)
	}

	updated, _ := st.LimitOrder().GetLimitOrder(order.ID)
	if updated.Status != "filled" {
		t.Errorf("Expected status 'filled', got '%s'", updated.Status)
	}
	if updated.FilledPrice != 49500.50 {
		t.Errorf("Expected filled price 49500.50, got %f", updated.FilledPrice)
	}
	if updated.FilledQuantity != 0.1 {
		t.Errorf("Expected filled quantity 0.1, got %f", updated.FilledQuantity)
	}
	if updated.Commission != 0.005 {
		t.Errorf("Expected commission 0.005, got %f", updated.Commission)
	}
}

func TestLimitOrderEngine_ConcurrentRegistration(t *testing.T) {
	st := setupTestStore(t)
	engine := NewLimitOrderEngine(st)

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			traderID := fmt.Sprintf("trader-%d", id)
			engine.RegisterTrader(traderID, nil)
			time.Sleep(10 * time.Millisecond)
			engine.UnregisterTrader(traderID)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	engine.tradersMu.RLock()
	count := len(engine.traders)
	engine.tradersMu.RUnlock()

	if count != 0 {
		t.Errorf("Expected 0 traders after concurrent ops, got %d", count)
	}
}

func TestLimitOrderEngine_ExecuteOrder_UnregisteredCancels(t *testing.T) {
	st := setupTestStore(t)
	if err := st.GormDB().AutoMigrate(&store.LimitOrderModel{}); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}
	engine := NewLimitOrderEngine(st)

	order := &store.LimitOrderModel{
		TraderID:         "trader-1",
		Symbol:           "BTCUSDT",
		Side:             "BUY",
		PositionSide:     "LONG",
		TriggerCondition: "lte",
		TriggerPrice:     50000,
		Quantity:         0.1,
		Status:           "pending",
	}

	if err := st.LimitOrder().CreateLimitOrder(order); err != nil {
		t.Fatalf("CreateLimitOrder failed: %v", err)
	}

	engine.executeOrder(order, 49000)

	updated, err := st.LimitOrder().GetLimitOrder(order.ID)
	if err != nil {
		t.Fatalf("GetLimitOrder failed: %v", err)
	}
	if updated.Status != "canceled" {
		t.Errorf("Expected status canceled, got %s", updated.Status)
	}
}

func TestLimitOrderEngine_ExecuteOrder_ErrorHandling(t *testing.T) {
	execErr := errors.New("exec failed")

	type expectedCalls struct {
		openLong   int
		openShort  int
		closeLong  int
		closeShort int
	}

	testCases := []struct {
		name         string
		side         string
		positionSide string
		setErr       func(*mockTrader)
		wantCalls    expectedCalls
	}{
		{
			name:         "BUY LONG OpenLong error",
			side:         "BUY",
			positionSide: "LONG",
			setErr:       func(m *mockTrader) { m.openLongErr = execErr },
			wantCalls:    expectedCalls{openLong: 1},
		},
		{
			name:         "BUY SHORT CloseShort error",
			side:         "BUY",
			positionSide: "SHORT",
			setErr:       func(m *mockTrader) { m.closeShortErr = execErr },
			wantCalls:    expectedCalls{closeShort: 1},
		},
		{
			name:         "SELL SHORT OpenShort error",
			side:         "SELL",
			positionSide: "SHORT",
			setErr:       func(m *mockTrader) { m.openShortErr = execErr },
			wantCalls:    expectedCalls{openShort: 1},
		},
		{
			name:         "SELL LONG CloseLong error",
			side:         "SELL",
			positionSide: "LONG",
			setErr:       func(m *mockTrader) { m.closeLongErr = execErr },
			wantCalls:    expectedCalls{closeLong: 1},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			st := setupTestStore(t)
			if err := st.GormDB().AutoMigrate(&store.LimitOrderModel{}); err != nil {
				t.Fatalf("Failed to migrate: %v", err)
			}
			engine := NewLimitOrderEngine(st)

			mock := &mockTrader{}
			tc.setErr(mock)
			engine.RegisterTrader("trader-1", &AutoTrader{trader: mock})

			order := &store.LimitOrderModel{
				TraderID:         "trader-1",
				Symbol:           "BTCUSDT",
				Side:             tc.side,
				PositionSide:     tc.positionSide,
				TriggerCondition: "lte",
				TriggerPrice:     50000,
				Quantity:         0.1,
				Leverage:         2,
				Status:           "pending",
			}

			if err := st.LimitOrder().CreateLimitOrder(order); err != nil {
				t.Fatalf("CreateLimitOrder failed: %v", err)
			}

			engine.executeOrder(order, 49500)

			updated, err := st.LimitOrder().GetLimitOrder(order.ID)
			if err != nil {
				t.Fatalf("GetLimitOrder failed: %v", err)
			}
			if updated.Status != "failed" {
				t.Errorf("Expected status failed, got %s", updated.Status)
			}

			openLong, openShort, closeLong, closeShort := mock.calls()
			if openLong != tc.wantCalls.openLong ||
				openShort != tc.wantCalls.openShort ||
				closeLong != tc.wantCalls.closeLong ||
				closeShort != tc.wantCalls.closeShort {
				t.Errorf("Unexpected call counts: openLong=%d openShort=%d closeLong=%d closeShort=%d",
					openLong, openShort, closeLong, closeShort)
			}
		})
	}
}

func TestLimitOrderEngine_ExecuteOrder_ResultFields(t *testing.T) {
	testCases := []struct {
		name           string
		result         map[string]interface{}
		price          float64
		quantity       float64
		wantPrice      float64
		wantQuantity   float64
		wantCommission float64
		wantOrderID    string
	}{
		{
			name: "uses result fields",
			result: map[string]interface{}{
				"avgPrice":    49800.0,
				"executedQty": 0.2,
				"commission":  0.001,
				"orderId":     "ex-123",
			},
			price:          50000,
			quantity:       0.1,
			wantPrice:      49800.0,
			wantQuantity:   0.2,
			wantCommission: 0.001,
			wantOrderID:    "ex-123",
		},
		{
			name: "falls back to order values",
			result: map[string]interface{}{
				"avgPrice":    0.0,
				"executedQty": 0.0,
				"commission":  0.002,
				"orderId":     "ex-456",
			},
			price:          51000,
			quantity:       0.3,
			wantPrice:      51000,
			wantQuantity:   0.3,
			wantCommission: 0.002,
			wantOrderID:    "ex-456",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			st := setupTestStore(t)
			if err := st.GormDB().AutoMigrate(&store.LimitOrderModel{}); err != nil {
				t.Fatalf("Failed to migrate: %v", err)
			}
			engine := NewLimitOrderEngine(st)

			mock := &mockTrader{
				openLongResult: tc.result,
			}
			engine.RegisterTrader("trader-1", &AutoTrader{trader: mock})

			order := &store.LimitOrderModel{
				TraderID:         "trader-1",
				Symbol:           "BTCUSDT",
				Side:             "BUY",
				PositionSide:     "LONG",
				TriggerCondition: "lte",
				TriggerPrice:     tc.price,
				Quantity:         tc.quantity,
				Leverage:         5,
				Status:           "pending",
			}

			if err := st.LimitOrder().CreateLimitOrder(order); err != nil {
				t.Fatalf("CreateLimitOrder failed: %v", err)
			}

			engine.executeOrder(order, tc.price)

			updated, err := st.LimitOrder().GetLimitOrder(order.ID)
			if err != nil {
				t.Fatalf("GetLimitOrder failed: %v", err)
			}
			if updated.Status != "filled" {
				t.Errorf("Expected status filled, got %s", updated.Status)
			}
			if updated.FilledPrice != tc.wantPrice {
				t.Errorf("Expected filled price %.4f, got %.4f", tc.wantPrice, updated.FilledPrice)
			}
			if updated.FilledQuantity != tc.wantQuantity {
				t.Errorf("Expected filled quantity %.4f, got %.4f", tc.wantQuantity, updated.FilledQuantity)
			}
			if updated.Commission != tc.wantCommission {
				t.Errorf("Expected commission %.4f, got %.4f", tc.wantCommission, updated.Commission)
			}
			if updated.ExchangeOrderID != tc.wantOrderID {
				t.Errorf("Expected exchange order ID %s, got %s", tc.wantOrderID, updated.ExchangeOrderID)
			}
			if updated.TriggeredAt == nil {
				t.Error("TriggeredAt should be set for filled order")
			}

			openLong, openShort, closeLong, closeShort := mock.calls()
			if openLong != 1 || openShort != 0 || closeLong != 0 || closeShort != 0 {
				t.Errorf("Unexpected call counts: openLong=%d openShort=%d closeLong=%d closeShort=%d",
					openLong, openShort, closeLong, closeShort)
			}
		})
	}
}

func TestLimitOrderEngine_ConcurrentRegisterUnregister(t *testing.T) {
	st := setupTestStore(t)
	engine := NewLimitOrderEngine(st)

	const workers = 20
	const loops = 100

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		traderID := fmt.Sprintf("trader-%d", i)
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			for j := 0; j < loops; j++ {
				engine.RegisterTrader(id, nil)
				engine.UnregisterTrader(id)
			}
		}(traderID)
	}
	wg.Wait()

	for i := 0; i < workers; i++ {
		engine.UnregisterTrader(fmt.Sprintf("trader-%d", i))
	}

	engine.tradersMu.RLock()
	count := len(engine.traders)
	engine.tradersMu.RUnlock()

	if count != 0 {
		t.Errorf("Expected 0 registered traders, got %d", count)
	}
}
