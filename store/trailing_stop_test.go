//go:build cgo

package store

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupTestTrailingStopDB(t *testing.T) *TrailingStopStore {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	store := NewTrailingStopStore(db)
	if err := store.initTables(); err != nil {
		t.Fatalf("Failed to init tables: %v", err)
	}
	return store
}

func TestTrailingStopStore_CreateAndGet(t *testing.T) {
	store := setupTestTrailingStopDB(t)

	config := &TrailingStopConfigModel{
		TraderID:         "trader-1",
		Symbol:           "BTCUSDT",
		PositionSide:     "LONG",
		StopType:         "stop_loss",
		TrailingMode:     "percent",
		TrailingDistance: 2.0,
		ActivationPrice:  52000.0,
		Quantity:         0.1,
		EntryPrice:       50000.0,
		IsActive:         true,
		Status:           "active",
	}

	if err := store.CreateTrailingStop(config); err != nil {
		t.Fatalf("CreateTrailingStop failed: %v", err)
	}

	if config.ID == "" {
		t.Error("Config ID should be generated")
	}

	retrieved, err := store.GetTrailingStop(config.ID)
	if err != nil {
		t.Fatalf("GetTrailingStop failed: %v", err)
	}

	if retrieved.Symbol != "BTCUSDT" {
		t.Errorf("Expected symbol BTCUSDT, got %s", retrieved.Symbol)
	}
	if retrieved.TrailingDistance != 2.0 {
		t.Errorf("Expected trailing distance 2.0, got %.2f", retrieved.TrailingDistance)
	}
	if retrieved.StopType != "stop_loss" {
		t.Errorf("Expected stop_loss, got %s", retrieved.StopType)
	}
}

func TestTrailingStopStore_GetActiveStops(t *testing.T) {
	store := setupTestTrailingStopDB(t)

	configs := []*TrailingStopConfigModel{
		{TraderID: "trader-1", Symbol: "BTCUSDT", PositionSide: "LONG", StopType: "stop_loss", TrailingMode: "percent", TrailingDistance: 2.0, Quantity: 0.1, IsActive: true, Status: "active"},
		{TraderID: "trader-1", Symbol: "ETHUSDT", PositionSide: "SHORT", StopType: "take_profit", TrailingMode: "fixed_points", TrailingDistance: 100, Quantity: 1.0, IsActive: true, Status: "active"},
		{TraderID: "trader-1", Symbol: "XRPUSDT", PositionSide: "LONG", StopType: "stop_loss", TrailingMode: "percent", TrailingDistance: 3.0, Quantity: 100, IsActive: false, Status: "triggered"},
		{TraderID: "trader-2", Symbol: "BTCUSDT", PositionSide: "LONG", StopType: "stop_loss", TrailingMode: "percent", TrailingDistance: 1.5, Quantity: 0.2, IsActive: true, Status: "active"},
	}

	for _, c := range configs {
		if err := store.CreateTrailingStop(c); err != nil {
			t.Fatalf("CreateTrailingStop failed: %v", err)
		}
	}

	active, err := store.GetActiveTrailingStops("trader-1")
	if err != nil {
		t.Fatalf("GetActiveTrailingStops failed: %v", err)
	}
	if len(active) != 2 {
		t.Errorf("Expected 2 active configs for trader-1, got %d", len(active))
	}

	allActive, err := store.GetAllActiveTrailingStops()
	if err != nil {
		t.Fatalf("GetAllActiveTrailingStops failed: %v", err)
	}
	if len(allActive) != 3 {
		t.Errorf("Expected 3 total active configs, got %d", len(allActive))
	}
}

func TestTrailingStopStore_UpdatePrice(t *testing.T) {
	store := setupTestTrailingStopDB(t)

	config := &TrailingStopConfigModel{
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

	store.CreateTrailingStop(config)

	err := store.UpdateTrailingStopPrice(config.ID, 49000.0, 52000.0, 48000.0)
	if err != nil {
		t.Fatalf("UpdateTrailingStopPrice failed: %v", err)
	}

	updated, _ := store.GetTrailingStop(config.ID)
	if updated.CurrentStopPrice != 49000.0 {
		t.Errorf("Expected current stop 49000, got %.2f", updated.CurrentStopPrice)
	}
	if updated.HighestPrice != 52000.0 {
		t.Errorf("Expected highest 52000, got %.2f", updated.HighestPrice)
	}
	if updated.LowestPrice != 48000.0 {
		t.Errorf("Expected lowest 48000, got %.2f", updated.LowestPrice)
	}
}

func TestTrailingStopStore_Trigger(t *testing.T) {
	store := setupTestTrailingStopDB(t)

	config := &TrailingStopConfigModel{
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

	store.CreateTrailingStop(config)

	triggeredPrice := 48500.0
	if err := store.TriggerTrailingStop(config.ID, triggeredPrice); err != nil {
		t.Fatalf("TriggerTrailingStop failed: %v", err)
	}

	triggered, _ := store.GetTrailingStop(config.ID)
	if triggered.Status != "triggered" {
		t.Errorf("Expected status triggered, got %s", triggered.Status)
	}
	if triggered.IsActive {
		t.Error("IsActive should be false after trigger")
	}
	if triggered.TriggeredPrice != triggeredPrice {
		t.Errorf("Expected triggered price %.2f, got %.2f", triggeredPrice, triggered.TriggeredPrice)
	}
	if triggered.TriggeredAt == nil {
		t.Error("TriggeredAt should be set")
	}
}

func TestTrailingStopStore_Cancel(t *testing.T) {
	store := setupTestTrailingStopDB(t)

	config := &TrailingStopConfigModel{
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

	store.CreateTrailingStop(config)

	if err := store.CancelTrailingStop(config.ID); err != nil {
		t.Fatalf("CancelTrailingStop failed: %v", err)
	}

	canceled, _ := store.GetTrailingStop(config.ID)
	if canceled.Status != "canceled" {
		t.Errorf("Expected status canceled, got %s", canceled.Status)
	}
	if canceled.IsActive {
		t.Error("IsActive should be false after cancel")
	}
}

func TestTrailingStopStore_GetWithStatusFilter(t *testing.T) {
	store := setupTestTrailingStopDB(t)

	configs := []*TrailingStopConfigModel{
		{TraderID: "trader-1", Symbol: "BTCUSDT", PositionSide: "LONG", StopType: "stop_loss", TrailingMode: "percent", TrailingDistance: 2.0, Quantity: 0.1, IsActive: true, Status: "active"},
		{TraderID: "trader-1", Symbol: "ETHUSDT", PositionSide: "SHORT", StopType: "take_profit", TrailingMode: "fixed_points", TrailingDistance: 100, Quantity: 1.0, IsActive: false, Status: "triggered"},
		{TraderID: "trader-1", Symbol: "XRPUSDT", PositionSide: "LONG", StopType: "stop_loss", TrailingMode: "percent", TrailingDistance: 3.0, Quantity: 100, IsActive: false, Status: "canceled"},
	}

	for _, c := range configs {
		store.CreateTrailingStop(c)
	}

	activeConfigs, err := store.GetTrailingStops("trader-1", "active")
	if err != nil {
		t.Fatalf("GetTrailingStops failed: %v", err)
	}
	if len(activeConfigs) != 1 {
		t.Errorf("Expected 1 active config, got %d", len(activeConfigs))
	}

	allConfigs, err := store.GetTrailingStops("trader-1", "")
	if err != nil {
		t.Fatalf("GetTrailingStops failed: %v", err)
	}
	if len(allConfigs) != 3 {
		t.Errorf("Expected 3 total configs, got %d", len(allConfigs))
	}
}

func TestTrailingStopStore_TrailingModes(t *testing.T) {
	store := setupTestTrailingStopDB(t)

	percentConfig := &TrailingStopConfigModel{
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
	store.CreateTrailingStop(percentConfig)

	fixedConfig := &TrailingStopConfigModel{
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
	store.CreateTrailingStop(fixedConfig)

	retrieved1, _ := store.GetTrailingStop(percentConfig.ID)
	if retrieved1.TrailingMode != "percent" {
		t.Errorf("Expected percent mode, got %s", retrieved1.TrailingMode)
	}

	retrieved2, _ := store.GetTrailingStop(fixedConfig.ID)
	if retrieved2.TrailingMode != "fixed_points" {
		t.Errorf("Expected fixed_points mode, got %s", retrieved2.TrailingMode)
	}
}

func TestTrailingStopStore_Timestamps(t *testing.T) {
	store := setupTestTrailingStopDB(t)

	config := &TrailingStopConfigModel{
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

	beforeCreate := time.Now().Add(-1 * time.Second)
	store.CreateTrailingStop(config)

	retrieved, _ := store.GetTrailingStop(config.ID)
	if retrieved.CreatedAt.Before(beforeCreate) {
		t.Error("CreatedAt should be after test start")
	}

	time.Sleep(10 * time.Millisecond)
	store.UpdateTrailingStopPrice(config.ID, 49000, 52000, 48000)

	updated, _ := store.GetTrailingStop(config.ID)
	if !updated.UpdatedAt.After(retrieved.CreatedAt) {
		t.Error("UpdatedAt should be after CreatedAt after update")
	}
}
