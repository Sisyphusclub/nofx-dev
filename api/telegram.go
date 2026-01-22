package api

import (
	"net/http"
	"nofx/store"
	"nofx/telegram"

	"github.com/gin-gonic/gin"
)

type TelegramSettingsRequest struct {
	ChatID       int64 `json:"chat_id"`
	Enabled      bool  `json:"enabled"`
	NotifyOpen   bool  `json:"notify_open"`
	NotifyClose  bool  `json:"notify_close"`
	NotifyErrors bool  `json:"notify_errors"`
}

func (s *Server) handleGetTelegramSettings(c *gin.Context) {
	userID := c.GetString("userID")
	traderID := c.Param("trader_id")

	settings, err := s.store.Telegram().GetSettings(userID, traderID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if settings == nil {
		settings = &store.TelegramSettings{
			UserID:       userID,
			TraderID:     traderID,
			Enabled:      false,
			NotifyOpen:   true,
			NotifyClose:  true,
			NotifyErrors: false,
		}
	}

	c.JSON(http.StatusOK, settings)
}

func (s *Server) handleGetAllTelegramSettings(c *gin.Context) {
	userID := c.GetString("userID")

	settings, err := s.store.Telegram().GetAllForUser(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, settings)
}

func (s *Server) handleUpdateTelegramSettings(c *gin.Context) {
	userID := c.GetString("userID")
	traderID := c.Param("trader_id")

	var req TelegramSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	settings, err := s.store.Telegram().GetSettings(userID, traderID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if settings == nil {
		settings = &store.TelegramSettings{
			UserID:   userID,
			TraderID: traderID,
		}
	}

	settings.ChatID = req.ChatID
	settings.Enabled = req.Enabled
	settings.NotifyOpen = req.NotifyOpen
	settings.NotifyClose = req.NotifyClose
	settings.NotifyErrors = req.NotifyErrors

	if err := s.store.Telegram().SaveSettings(settings); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, settings)
}

type TestNotificationRequest struct {
	TraderID string `json:"trader_id"`
	Message  string `json:"message"`
}

func (s *Server) handleTestTelegramNotification(c *gin.Context) {
	userID := c.GetString("userID")

	var req TestNotificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	settings, err := s.store.Telegram().GetSettings(userID, req.TraderID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if settings == nil || settings.ChatID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Telegram chat ID not configured"})
		return
	}

	bot := telegram.GetBot()
	if bot == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Telegram bot not initialized"})
		return
	}

	message := req.Message
	if message == "" {
		message = "🔔 Test notification from NOFX AI Trader"
	}

	if err := bot.Notifier().Send(settings.ChatID, message); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}
