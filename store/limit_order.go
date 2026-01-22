package store

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// LimitOrderModel represents a limit order.
type LimitOrderModel struct {
	ID               string     `json:"id" gorm:"primaryKey"`
	TraderID         string     `json:"trader_id" gorm:"index;not null"`
	ExchangeID       string     `json:"exchange_id" gorm:"index"`
	Symbol           string     `json:"symbol" gorm:"not null"`
	Side             string     `json:"side" gorm:"not null"`              // BUY/SELL
	PositionSide     string     `json:"position_side"`                     // LONG/SHORT
	TriggerCondition string     `json:"trigger_condition" gorm:"not null"` // lte/gte
	TriggerPrice     float64    `json:"trigger_price" gorm:"not null"`
	Quantity         float64    `json:"quantity" gorm:"not null"`
	Leverage         int        `json:"leverage" gorm:"default:1"`
	Status           string     `json:"status" gorm:"default:pending;index"` // pending/filled/canceled/expired
	FilledPrice      float64    `json:"filled_price"`
	FilledQuantity   float64    `json:"filled_quantity"`
	Commission       float64    `json:"commission"`
	ExchangeOrderID  string     `json:"exchange_order_id"`
	ErrorMessage     string     `json:"error_message"`
	CreatedAt        time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt        time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
	TriggeredAt      *time.Time `json:"triggered_at,omitempty"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty"`
}

func (LimitOrderModel) TableName() string {
	return "limit_orders"
}

// LimitOrderStore limit order storage.
type LimitOrderStore struct {
	db *gorm.DB
}

// NewLimitOrderStore creates a new limit order store.
func NewLimitOrderStore(db *gorm.DB) *LimitOrderStore {
	return &LimitOrderStore{db: db}
}

func (s *LimitOrderStore) initTables() error {
	if s.db.Dialector.Name() == "postgres" {
		var tableExists int64
		s.db.Raw(`SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'limit_orders'`).Scan(&tableExists)
		if tableExists > 0 {
			return nil
		}
	}
	return s.db.AutoMigrate(&LimitOrderModel{})
}

// CreateLimitOrder creates a new limit order.
func (s *LimitOrderStore) CreateLimitOrder(order *LimitOrderModel) error {
	if order.ID == "" {
		order.ID = uuid.New().String()
	}
	return s.db.Create(order).Error
}

// GetLimitOrder gets a limit order by ID.
func (s *LimitOrderStore) GetLimitOrder(id string) (*LimitOrderModel, error) {
	var order LimitOrderModel
	if err := s.db.Where("id = ?", id).First(&order).Error; err != nil {
		return nil, err
	}
	return &order, nil
}

// GetPendingLimitOrders gets all pending limit orders for a trader.
func (s *LimitOrderStore) GetPendingLimitOrders(traderID string) ([]LimitOrderModel, error) {
	var orders []LimitOrderModel
	err := s.db.Where("trader_id = ? AND status = ?", traderID, "pending").
		Order("created_at DESC").
		Find(&orders).Error
	if err != nil {
		return nil, err
	}
	return orders, nil
}

// GetAllPendingOrders gets all pending limit orders across traders.
func (s *LimitOrderStore) GetAllPendingOrders() ([]LimitOrderModel, error) {
	var orders []LimitOrderModel
	err := s.db.Where("status = ?", "pending").
		Order("created_at DESC").
		Find(&orders).Error
	if err != nil {
		return nil, err
	}
	return orders, nil
}

// UpdateLimitOrderStatus updates limit order status and fill details.
func (s *LimitOrderStore) UpdateLimitOrderStatus(id, status string, filledPrice, filledQty, commission float64) error {
	updates := map[string]interface{}{
		"status":          status,
		"filled_price":    filledPrice,
		"filled_quantity": filledQty,
		"commission":      commission,
		"updated_at":      time.Now().UTC(),
	}
	return s.db.Model(&LimitOrderModel{}).Where("id = ?", id).Updates(updates).Error
}

// CancelLimitOrder cancels a limit order.
func (s *LimitOrderStore) CancelLimitOrder(id string) error {
	return s.db.Model(&LimitOrderModel{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":     "canceled",
		"updated_at": time.Now().UTC(),
	}).Error
}

// ExpireLimitOrders marks expired pending orders and returns affected rows.
func (s *LimitOrderStore) ExpireLimitOrders() (int64, error) {
	now := time.Now().UTC()
	result := s.db.Model(&LimitOrderModel{}).
		Where("status = ? AND expires_at IS NOT NULL AND expires_at <= ?", "pending", now).
		Updates(map[string]interface{}{
			"status":     "expired",
			"updated_at": now,
		})
	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}

// GetLimitOrders gets limit orders for a trader with optional filters.
func (s *LimitOrderStore) GetLimitOrders(traderID string, symbol string, status string) ([]LimitOrderModel, error) {
	var orders []LimitOrderModel
	query := s.db.Where("trader_id = ?", traderID)
	if symbol != "" {
		query = query.Where("symbol = ?", symbol)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	err := query.Order("created_at DESC").Find(&orders).Error
	if err != nil {
		return nil, err
	}
	return orders, nil
}
