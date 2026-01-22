package api

import (
	"net/http"
	"nofx/store"
	"time"

	"github.com/gin-gonic/gin"
)

// Limit Order Handlers

// handleCreateLimitOrder creates a new limit order
func (s *Server) handleCreateLimitOrder(c *gin.Context) {
	userID, _ := c.Get("user_id")

	var req struct {
		TraderID         string   `json:"trader_id" binding:"required"`
		Symbol           string   `json:"symbol" binding:"required"`
		Side             string   `json:"side" binding:"required,oneof=BUY SELL"`
		PositionSide     string   `json:"position_side" binding:"required,oneof=LONG SHORT"`
		TriggerCondition string   `json:"trigger_condition" binding:"required,oneof=lte gte"`
		TriggerPrice     float64  `json:"trigger_price" binding:"required,gt=0"`
		Quantity         float64  `json:"quantity" binding:"required,gt=0"`
		Leverage         int      `json:"leverage"`
		ExpiresIn        *int     `json:"expires_in"` // minutes, optional
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify trader belongs to user
	_, err := s.store.Trader().GetFullConfig(userID.(string), req.TraderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "trader not found"})
		return
	}

	// Get trader config for exchange ID
	traders, _ := s.store.Trader().List(userID.(string))
	var traderExchangeID string
	for _, t := range traders {
		if t.ID == req.TraderID {
			traderExchangeID = t.ExchangeID
			break
		}
	}

	order := &store.LimitOrderModel{
		TraderID:         req.TraderID,
		ExchangeID:       traderExchangeID,
		Symbol:           req.Symbol,
		Side:             req.Side,
		PositionSide:     req.PositionSide,
		TriggerCondition: req.TriggerCondition,
		TriggerPrice:     req.TriggerPrice,
		Quantity:         req.Quantity,
		Leverage:         req.Leverage,
		Status:           "pending",
	}

	if req.Leverage == 0 {
		order.Leverage = 1
	}

	if req.ExpiresIn != nil && *req.ExpiresIn > 0 {
		expiresAt := time.Now().UTC().Add(time.Duration(*req.ExpiresIn) * time.Minute)
		order.ExpiresAt = &expiresAt
	}

	if err := s.store.LimitOrder().CreateLimitOrder(order); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, order)
}

// handleGetLimitOrders gets limit orders for a trader
func (s *Server) handleGetLimitOrders(c *gin.Context) {
	userID, _ := c.Get("user_id")
	traderID := c.Query("trader_id")
	symbol := c.Query("symbol")
	status := c.Query("status")

	if traderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "trader_id required"})
		return
	}

	// Verify trader belongs to user
	_, err := s.store.Trader().GetFullConfig(userID.(string), traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "trader not found"})
		return
	}

	orders, err := s.store.LimitOrder().GetLimitOrders(traderID, symbol, status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, orders)
}

// handleGetLimitOrder gets a specific limit order
func (s *Server) handleGetLimitOrder(c *gin.Context) {
	userID, _ := c.Get("user_id")
	orderID := c.Param("id")

	order, err := s.store.LimitOrder().GetLimitOrder(orderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
		return
	}

	// Verify trader belongs to user
	_, err = s.store.Trader().GetFullConfig(userID.(string), order.TraderID)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	c.JSON(http.StatusOK, order)
}

// handleCancelLimitOrder cancels a limit order
func (s *Server) handleCancelLimitOrder(c *gin.Context) {
	userID, _ := c.Get("user_id")
	orderID := c.Param("id")

	order, err := s.store.LimitOrder().GetLimitOrder(orderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
		return
	}

	// Verify trader belongs to user
	_, err = s.store.Trader().GetFullConfig(userID.(string), order.TraderID)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	if order.Status != "pending" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "can only cancel pending orders"})
		return
	}

	if err := s.store.LimitOrder().CancelLimitOrder(orderID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "order canceled"})
}

// Trailing Stop Handlers

// handleCreateTrailingStop creates a new trailing stop configuration
func (s *Server) handleCreateTrailingStop(c *gin.Context) {
	userID, _ := c.Get("user_id")

	var req struct {
		TraderID         string  `json:"trader_id" binding:"required"`
		Symbol           string  `json:"symbol" binding:"required"`
		PositionSide     string  `json:"position_side" binding:"required,oneof=LONG SHORT"`
		StopType         string  `json:"stop_type" binding:"required,oneof=stop_loss take_profit"`
		TrailingMode     string  `json:"trailing_mode" binding:"required,oneof=percent fixed_points"`
		TrailingDistance float64 `json:"trailing_distance" binding:"required,gt=0"`
		ActivationPrice  float64 `json:"activation_price"`
		Quantity         float64 `json:"quantity" binding:"required,gt=0"`
		EntryPrice       float64 `json:"entry_price"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify trader belongs to user
	_, err := s.store.Trader().GetFullConfig(userID.(string), req.TraderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "trader not found"})
		return
	}

	config := &store.TrailingStopConfigModel{
		TraderID:         req.TraderID,
		Symbol:           req.Symbol,
		PositionSide:     req.PositionSide,
		StopType:         req.StopType,
		TrailingMode:     req.TrailingMode,
		TrailingDistance: req.TrailingDistance,
		ActivationPrice:  req.ActivationPrice,
		Quantity:         req.Quantity,
		EntryPrice:       req.EntryPrice,
		IsActive:         true,
		Status:           "active",
	}

	if err := s.store.TrailingStop().CreateTrailingStop(config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, config)
}

// handleGetTrailingStops gets trailing stops for a trader
func (s *Server) handleGetTrailingStops(c *gin.Context) {
	userID, _ := c.Get("user_id")
	traderID := c.Query("trader_id")
	status := c.Query("status")

	if traderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "trader_id required"})
		return
	}

	// Verify trader belongs to user
	_, err := s.store.Trader().GetFullConfig(userID.(string), traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "trader not found"})
		return
	}

	configs, err := s.store.TrailingStop().GetTrailingStops(traderID, status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, configs)
}

// handleGetTrailingStop gets a specific trailing stop configuration
func (s *Server) handleGetTrailingStop(c *gin.Context) {
	userID, _ := c.Get("user_id")
	configID := c.Param("id")

	config, err := s.store.TrailingStop().GetTrailingStop(configID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "trailing stop not found"})
		return
	}

	// Verify trader belongs to user
	_, err = s.store.Trader().GetFullConfig(userID.(string), config.TraderID)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	c.JSON(http.StatusOK, config)
}

// handleCancelTrailingStop cancels a trailing stop
func (s *Server) handleCancelTrailingStop(c *gin.Context) {
	userID, _ := c.Get("user_id")
	configID := c.Param("id")

	config, err := s.store.TrailingStop().GetTrailingStop(configID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "trailing stop not found"})
		return
	}

	// Verify trader belongs to user
	_, err = s.store.Trader().GetFullConfig(userID.(string), config.TraderID)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	if config.Status != "active" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "can only cancel active trailing stops"})
		return
	}

	if err := s.store.TrailingStop().CancelTrailingStop(configID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "trailing stop canceled"})
}
