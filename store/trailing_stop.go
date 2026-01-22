package store

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// TrailingStopConfigModel represents trailing stop configuration.
type TrailingStopConfigModel struct {
	ID               string     `json:"id" gorm:"primaryKey"`
	TraderID         string     `json:"trader_id" gorm:"index;not null"`
	Symbol           string     `json:"symbol" gorm:"not null"`
	PositionSide     string     `json:"position_side" gorm:"not null"` // LONG/SHORT
	StopType         string     `json:"stop_type" gorm:"not null"`     // stop_loss/take_profit
	TrailingMode     string     `json:"trailing_mode" gorm:"not null"` // percent/fixed_points
	TrailingDistance float64    `json:"trailing_distance" gorm:"not null"`
	ActivationPrice  float64    `json:"activation_price"`
	Quantity         float64    `json:"quantity" gorm:"not null"`
	IsActive         bool       `json:"is_active" gorm:"default:true"`
	CurrentStopPrice float64    `json:"current_stop_price"`
	HighestPrice     float64    `json:"highest_price"`
	LowestPrice      float64    `json:"lowest_price"`
	EntryPrice       float64    `json:"entry_price"`
	Status           string     `json:"status" gorm:"default:active"` // active/triggered/canceled
	TriggeredAt      *time.Time `json:"triggered_at,omitempty"`
	TriggeredPrice   float64    `json:"triggered_price"`
	CreatedAt        time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt        time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (TrailingStopConfigModel) TableName() string {
	return "trailing_stop_configs"
}

// TrailingStopStore trailing stop storage.
type TrailingStopStore struct {
	db *gorm.DB
}

// NewTrailingStopStore creates a new trailing stop store.
func NewTrailingStopStore(db *gorm.DB) *TrailingStopStore {
	return &TrailingStopStore{db: db}
}

func (s *TrailingStopStore) initTables() error {
	if s.db.Dialector.Name() == "postgres" {
		var tableExists int64
		s.db.Raw(`SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'trailing_stop_configs'`).Scan(&tableExists)
		if tableExists > 0 {
			return nil
		}
	}
	return s.db.AutoMigrate(&TrailingStopConfigModel{})
}

// CreateTrailingStop creates a trailing stop configuration.
func (s *TrailingStopStore) CreateTrailingStop(config *TrailingStopConfigModel) error {
	if config.ID == "" {
		config.ID = uuid.New().String()
	}
	return s.db.Create(config).Error
}

// GetTrailingStop gets a trailing stop by ID.
func (s *TrailingStopStore) GetTrailingStop(id string) (*TrailingStopConfigModel, error) {
	var config TrailingStopConfigModel
	if err := s.db.Where("id = ?", id).First(&config).Error; err != nil {
		return nil, err
	}
	return &config, nil
}

// GetActiveTrailingStops gets active trailing stops for a trader.
func (s *TrailingStopStore) GetActiveTrailingStops(traderID string) ([]TrailingStopConfigModel, error) {
	var configs []TrailingStopConfigModel
	err := s.db.Where("trader_id = ? AND is_active = ? AND status = ?", traderID, true, "active").
		Order("created_at DESC").
		Find(&configs).Error
	if err != nil {
		return nil, err
	}
	return configs, nil
}

// GetAllActiveTrailingStops gets all active trailing stops.
func (s *TrailingStopStore) GetAllActiveTrailingStops() ([]TrailingStopConfigModel, error) {
	var configs []TrailingStopConfigModel
	err := s.db.Where("is_active = ? AND status = ?", true, "active").
		Order("created_at DESC").
		Find(&configs).Error
	if err != nil {
		return nil, err
	}
	return configs, nil
}

// UpdateTrailingStopPrice updates the current stop and extremes.
func (s *TrailingStopStore) UpdateTrailingStopPrice(id string, currentStop, highestPrice, lowestPrice float64) error {
	updates := map[string]interface{}{
		"current_stop_price": currentStop,
		"highest_price":      highestPrice,
		"lowest_price":       lowestPrice,
		"updated_at":         time.Now().UTC(),
	}
	return s.db.Model(&TrailingStopConfigModel{}).Where("id = ?", id).Updates(updates).Error
}

// TriggerTrailingStop marks a trailing stop as triggered.
func (s *TrailingStopStore) TriggerTrailingStop(id string, triggeredPrice float64) error {
	now := time.Now().UTC()
	updates := map[string]interface{}{
		"status":          "triggered",
		"is_active":       false,
		"triggered_price": triggeredPrice,
		"triggered_at":    now,
		"updated_at":      now,
	}
	return s.db.Model(&TrailingStopConfigModel{}).Where("id = ?", id).Updates(updates).Error
}

// CancelTrailingStop cancels a trailing stop.
func (s *TrailingStopStore) CancelTrailingStop(id string) error {
	return s.db.Model(&TrailingStopConfigModel{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":     "canceled",
		"is_active":  false,
		"updated_at": time.Now().UTC(),
	}).Error
}

// GetTrailingStops gets trailing stops for a trader with optional status filter.
func (s *TrailingStopStore) GetTrailingStops(traderID string, status string) ([]TrailingStopConfigModel, error) {
	var configs []TrailingStopConfigModel
	query := s.db.Where("trader_id = ?", traderID)
	if status != "" {
		query = query.Where("status = ?", status)
	}
	err := query.Order("created_at DESC").Find(&configs).Error
	if err != nil {
		return nil, err
	}
	return configs, nil
}
