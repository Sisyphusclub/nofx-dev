//go:build cgo

package store

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupTelegramTestStore(t *testing.T) *TelegramStore {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	if err := db.AutoMigrate(&TelegramSettings{}); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}
	return NewTelegramStore(db)
}

func TestTelegramStore_SaveAndGetSettings(t *testing.T) {
	store := setupTelegramTestStore(t)

	settings := &TelegramSettings{
		UserID:       "user-1",
		TraderID:     "trader-1",
		ChatID:       12345678,
		Enabled:      true,
		NotifyOpen:   true,
		NotifyClose:  true,
		NotifyErrors: false,
	}

	if err := store.SaveSettings(settings); err != nil {
		t.Fatalf("SaveSettings failed: %v", err)
	}

	retrieved, err := store.GetSettings("user-1", "trader-1")
	if err != nil {
		t.Fatalf("GetSettings failed: %v", err)
	}
	if retrieved == nil {
		t.Fatal("Expected settings, got nil")
	}
	if retrieved.ChatID != 12345678 {
		t.Errorf("Expected ChatID 12345678, got %d", retrieved.ChatID)
	}
	if !retrieved.Enabled {
		t.Error("Expected Enabled=true")
	}
}

func TestTelegramStore_GetSettingsNotFound(t *testing.T) {
	store := setupTelegramTestStore(t)

	settings, err := store.GetSettings("nonexistent", "trader")
	if err != nil {
		t.Fatalf("GetSettings failed: %v", err)
	}
	if settings != nil {
		t.Error("Expected nil for nonexistent settings")
	}
}

func TestTelegramStore_UpdateSettings(t *testing.T) {
	store := setupTelegramTestStore(t)

	settings := &TelegramSettings{
		UserID:      "user-1",
		TraderID:    "trader-1",
		ChatID:      12345678,
		Enabled:     true,
		NotifyOpen:  true,
		NotifyClose: true,
	}
	store.SaveSettings(settings)

	settings.ChatID = 87654321
	settings.NotifyErrors = true
	if err := store.SaveSettings(settings); err != nil {
		t.Fatalf("SaveSettings update failed: %v", err)
	}

	updated, _ := store.GetSettings("user-1", "trader-1")
	if updated.ChatID != 87654321 {
		t.Errorf("Expected ChatID 87654321, got %d", updated.ChatID)
	}
	if !updated.NotifyErrors {
		t.Error("Expected NotifyErrors=true")
	}
}

func TestTelegramStore_GetAllForUser(t *testing.T) {
	store := setupTelegramTestStore(t)

	for i := 0; i < 3; i++ {
		settings := &TelegramSettings{
			UserID:   "user-1",
			TraderID: string(rune('a' + i)),
			ChatID:   int64(1000 + i),
			Enabled:  true,
		}
		store.SaveSettings(settings)
	}

	otherSettings := &TelegramSettings{
		UserID:   "user-2",
		TraderID: "other",
		ChatID:   9999,
		Enabled:  true,
	}
	store.SaveSettings(otherSettings)

	list, err := store.GetAllForUser("user-1")
	if err != nil {
		t.Fatalf("GetAllForUser failed: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("Expected 3 settings for user-1, got %d", len(list))
	}
}

func TestTelegramStore_GetByTraderID(t *testing.T) {
	store := setupTelegramTestStore(t)

	settings := &TelegramSettings{
		UserID:   "user-1",
		TraderID: "unique-trader",
		ChatID:   12345,
		Enabled:  true,
	}
	store.SaveSettings(settings)

	found, err := store.GetByTraderID("unique-trader")
	if err != nil {
		t.Fatalf("GetByTraderID failed: %v", err)
	}
	if found == nil {
		t.Fatal("Expected settings, got nil")
	}
	if found.UserID != "user-1" {
		t.Errorf("Expected UserID 'user-1', got '%s'", found.UserID)
	}

	notFound, err := store.GetByTraderID("nonexistent")
	if err != nil {
		t.Fatalf("GetByTraderID failed: %v", err)
	}
	if notFound != nil {
		t.Error("Expected nil for nonexistent trader")
	}
}

func TestTelegramStore_SetDefaultTrader(t *testing.T) {
	store := setupTelegramTestStore(t)

	for _, id := range []string{"trader-1", "trader-2", "trader-3"} {
		settings := &TelegramSettings{
			UserID:        "user-1",
			TraderID:      id,
			ChatID:        12345,
			Enabled:       true,
			DefaultTrader: false,
		}
		store.SaveSettings(settings)
	}

	if err := store.SetDefaultTrader("user-1", "trader-2"); err != nil {
		t.Fatalf("SetDefaultTrader failed: %v", err)
	}

	defaultTrader, err := store.GetDefaultTrader("user-1")
	if err != nil {
		t.Fatalf("GetDefaultTrader failed: %v", err)
	}
	if defaultTrader == nil || defaultTrader.TraderID != "trader-2" {
		t.Error("Expected trader-2 as default")
	}

	if err := store.SetDefaultTrader("user-1", "trader-3"); err != nil {
		t.Fatalf("SetDefaultTrader failed: %v", err)
	}

	newDefault, _ := store.GetDefaultTrader("user-1")
	if newDefault == nil || newDefault.TraderID != "trader-3" {
		t.Error("Expected trader-3 as new default")
	}

	oldDefault, _ := store.GetSettings("user-1", "trader-2")
	if oldDefault.DefaultTrader {
		t.Error("Expected trader-2 to no longer be default")
	}
}

func TestTelegramStore_GetEnabledSettings(t *testing.T) {
	store := setupTelegramTestStore(t)

	enabledSettings := &TelegramSettings{
		UserID:   "user-1",
		TraderID: "enabled-trader",
		ChatID:   12345,
		Enabled:  true,
	}
	disabledSettings := &TelegramSettings{
		UserID:   "user-2",
		TraderID: "disabled-trader",
		ChatID:   54321,
		Enabled:  false,
	}
	store.SaveSettings(enabledSettings)
	store.SaveSettings(disabledSettings)

	enabled, err := store.GetEnabledSettings()
	if err != nil {
		t.Fatalf("GetEnabledSettings failed: %v", err)
	}
	if len(enabled) != 1 {
		t.Errorf("Expected 1 enabled settings, got %d", len(enabled))
	}
	if enabled[0].TraderID != "enabled-trader" {
		t.Errorf("Expected enabled-trader, got %s", enabled[0].TraderID)
	}
}

func TestTelegramStore_Delete(t *testing.T) {
	store := setupTelegramTestStore(t)

	settings := &TelegramSettings{
		UserID:   "user-1",
		TraderID: "trader-to-delete",
		ChatID:   12345,
		Enabled:  true,
	}
	store.SaveSettings(settings)

	if err := store.Delete("user-1", "trader-to-delete"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	deleted, _ := store.GetSettings("user-1", "trader-to-delete")
	if deleted != nil {
		t.Error("Expected settings to be deleted")
	}
}
