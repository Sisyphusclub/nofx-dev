//go:build cgo

package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"nofx/store"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupTelegramTestStore(t *testing.T) *store.Store {
	t.Helper()
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
	if err := st.GormDB().AutoMigrate(&store.TelegramSettings{}); err != nil {
		t.Fatalf("Failed to init telegram tables: %v", err)
	}
	return st
}

func newTelegramContext(t *testing.T, method, path, userID, traderID string, body []byte) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	req := httptest.NewRequest(method, path, bytes.NewBuffer(body))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	c.Request = req
	if traderID != "" {
		c.Params = []gin.Param{{Key: "trader_id", Value: traderID}}
	}
	c.Set("userID", userID)

	return c, w
}

func TestHandleGetTelegramSettings(t *testing.T) {
	userID := "user-1"
	traderID := "trader-1"

	testCases := []struct {
		name     string
		existing *store.TelegramSettings
		want     store.TelegramSettings
	}{
		{
			name:     "returns defaults when missing",
			existing: nil,
			want: store.TelegramSettings{
				UserID:       userID,
				TraderID:     traderID,
				Enabled:      false,
				NotifyOpen:   true,
				NotifyClose:  true,
				NotifyErrors: false,
			},
		},
		{
			name: "returns existing settings",
			existing: &store.TelegramSettings{
				UserID:       userID,
				TraderID:     traderID,
				ChatID:       1234,
				Enabled:      true,
				NotifyOpen:   false,
				NotifyClose:  true,
				NotifyErrors: true,
			},
			want: store.TelegramSettings{
				UserID:       userID,
				TraderID:     traderID,
				ChatID:       1234,
				Enabled:      true,
				NotifyOpen:   false,
				NotifyClose:  true,
				NotifyErrors: true,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			st := setupTelegramTestStore(t)
			srv := &Server{store: st}

			if tc.existing != nil {
				if err := st.Telegram().SaveSettings(tc.existing); err != nil {
					t.Fatalf("SaveSettings failed: %v", err)
				}
			}

			c, w := newTelegramContext(t, http.MethodGet, "/telegram/settings/"+traderID, userID, traderID, nil)
			srv.handleGetTelegramSettings(c)

			if w.Code != http.StatusOK {
				t.Fatalf("Expected status 200, got %d", w.Code)
			}

			var got store.TelegramSettings
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			if got.UserID != userID {
				t.Errorf("Expected user_id %s, got %s", userID, got.UserID)
			}
			if got.TraderID != traderID {
				t.Errorf("Expected trader_id %s, got %s", traderID, got.TraderID)
			}
			if got.ChatID != tc.want.ChatID {
				t.Errorf("Expected chat_id %d, got %d", tc.want.ChatID, got.ChatID)
			}
			if got.Enabled != tc.want.Enabled {
				t.Errorf("Expected enabled %v, got %v", tc.want.Enabled, got.Enabled)
			}
			if got.NotifyOpen != tc.want.NotifyOpen {
				t.Errorf("Expected notify_open %v, got %v", tc.want.NotifyOpen, got.NotifyOpen)
			}
			if got.NotifyClose != tc.want.NotifyClose {
				t.Errorf("Expected notify_close %v, got %v", tc.want.NotifyClose, got.NotifyClose)
			}
			if got.NotifyErrors != tc.want.NotifyErrors {
				t.Errorf("Expected notify_errors %v, got %v", tc.want.NotifyErrors, got.NotifyErrors)
			}
		})
	}
}

func TestHandleUpdateTelegramSettings(t *testing.T) {
	userID := "user-1"
	traderID := "trader-1"

	testCases := []struct {
		name     string
		existing *store.TelegramSettings
		req      TelegramSettingsRequest
		want     store.TelegramSettings
	}{
		{
			name: "creates new settings",
			req: TelegramSettingsRequest{
				ChatID:       1234,
				Enabled:      true,
				NotifyOpen:   true,
				NotifyClose:  false,
				NotifyErrors: true,
			},
			want: store.TelegramSettings{
				UserID:       userID,
				TraderID:     traderID,
				ChatID:       1234,
				Enabled:      true,
				NotifyOpen:   true,
				NotifyClose:  false,
				NotifyErrors: true,
			},
		},
		{
			name: "updates existing settings",
			existing: &store.TelegramSettings{
				UserID:       userID,
				TraderID:     traderID,
				ChatID:       1,
				Enabled:      false,
				NotifyOpen:   true,
				NotifyClose:  true,
				NotifyErrors: false,
			},
			req: TelegramSettingsRequest{
				ChatID:       9999,
				Enabled:      true,
				NotifyOpen:   false,
				NotifyClose:  true,
				NotifyErrors: true,
			},
			want: store.TelegramSettings{
				UserID:       userID,
				TraderID:     traderID,
				ChatID:       9999,
				Enabled:      true,
				NotifyOpen:   false,
				NotifyClose:  true,
				NotifyErrors: true,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			st := setupTelegramTestStore(t)
			srv := &Server{store: st}

			if tc.existing != nil {
				if err := st.Telegram().SaveSettings(tc.existing); err != nil {
					t.Fatalf("SaveSettings failed: %v", err)
				}
			}

			body, err := json.Marshal(tc.req)
			if err != nil {
				t.Fatalf("Failed to marshal request: %v", err)
			}

			c, w := newTelegramContext(t, http.MethodPut, "/telegram/settings/"+traderID, userID, traderID, body)
			srv.handleUpdateTelegramSettings(c)

			if w.Code != http.StatusOK {
				t.Fatalf("Expected status 200, got %d", w.Code)
			}

			var got store.TelegramSettings
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}
			if got.ChatID != tc.want.ChatID {
				t.Errorf("Expected chat_id %d, got %d", tc.want.ChatID, got.ChatID)
			}
			if got.Enabled != tc.want.Enabled {
				t.Errorf("Expected enabled %v, got %v", tc.want.Enabled, got.Enabled)
			}
			if got.NotifyOpen != tc.want.NotifyOpen {
				t.Errorf("Expected notify_open %v, got %v", tc.want.NotifyOpen, got.NotifyOpen)
			}
			if got.NotifyClose != tc.want.NotifyClose {
				t.Errorf("Expected notify_close %v, got %v", tc.want.NotifyClose, got.NotifyClose)
			}
			if got.NotifyErrors != tc.want.NotifyErrors {
				t.Errorf("Expected notify_errors %v, got %v", tc.want.NotifyErrors, got.NotifyErrors)
			}

			saved, err := st.Telegram().GetSettings(userID, traderID)
			if err != nil {
				t.Fatalf("GetSettings failed: %v", err)
			}
			if saved == nil {
				t.Fatal("Expected settings to be saved")
			}
			if saved.ChatID != tc.want.ChatID || saved.Enabled != tc.want.Enabled {
				t.Errorf("Saved settings mismatch: chat_id=%d enabled=%v", saved.ChatID, saved.Enabled)
			}
		})
	}
}

func TestHandleUpdateTelegramSettings_BadRequest(t *testing.T) {
	st := setupTelegramTestStore(t)
	srv := &Server{store: st}

	c, w := newTelegramContext(t, http.MethodPut, "/telegram/settings/trader-1", "user-1", "trader-1", []byte("{bad json"))
	srv.handleUpdateTelegramSettings(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleTestTelegramNotification_MissingChatID(t *testing.T) {
	st := setupTelegramTestStore(t)
	srv := &Server{store: st}

	body, _ := json.Marshal(TestNotificationRequest{
		TraderID: "trader-1",
		Message:  "test notification",
	})

	c, w := newTelegramContext(t, http.MethodPost, "/telegram/test", "user-1", "", body)
	srv.handleTestTelegramNotification(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected status 400, got %d", w.Code)
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if resp["error"] != "Telegram chat ID not configured" {
		t.Errorf("Expected 'Telegram chat ID not configured' error, got %q", resp["error"])
	}
}

func TestHandleTestTelegramNotification_BotNotInitialized(t *testing.T) {
	st := setupTelegramTestStore(t)
	srv := &Server{store: st}

	if err := st.Telegram().SaveSettings(&store.TelegramSettings{
		UserID:   "user-1",
		TraderID: "trader-1",
		ChatID:   1234,
	}); err != nil {
		t.Fatalf("SaveSettings failed: %v", err)
	}

	body, _ := json.Marshal(TestNotificationRequest{
		TraderID: "trader-1",
		Message:  "test notification",
	})

	c, w := newTelegramContext(t, http.MethodPost, "/telegram/test", "user-1", "", body)
	srv.handleTestTelegramNotification(c)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("Expected status 503, got %d", w.Code)
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if resp["error"] != "Telegram bot not initialized" {
		t.Errorf("Expected 'Telegram bot not initialized' error, got %q", resp["error"])
	}
}
