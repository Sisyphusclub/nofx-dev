package store

import (
	"time"

	"gorm.io/gorm"
)

type TelegramSettings struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	UserID        string    `gorm:"index" json:"user_id"`
	TraderID      string    `gorm:"index;uniqueIndex:idx_user_trader" json:"trader_id"`
	ChatID        int64     `json:"chat_id"`
	Enabled       bool      `gorm:"default:true" json:"enabled"`
	NotifyOpen    bool      `gorm:"default:true" json:"notify_open"`
	NotifyClose   bool      `gorm:"default:true" json:"notify_close"`
	NotifyErrors  bool      `gorm:"default:false" json:"notify_errors"`
	DefaultTrader bool      `gorm:"default:false" json:"default_trader"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (TelegramSettings) TableName() string {
	return "telegram_settings"
}

type TelegramStore struct {
	db *gorm.DB
}

func NewTelegramStore(db *gorm.DB) *TelegramStore {
	return &TelegramStore{db: db}
}

func (s *TelegramStore) initTables() error {
	return s.db.AutoMigrate(&TelegramSettings{})
}

func (s *TelegramStore) GetSettings(userID, traderID string) (*TelegramSettings, error) {
	var settings TelegramSettings
	err := s.db.Where("user_id = ? AND trader_id = ?", userID, traderID).First(&settings).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &settings, err
}

func (s *TelegramStore) SaveSettings(settings *TelegramSettings) error {
	return s.db.Save(settings).Error
}

func (s *TelegramStore) GetAllForUser(userID string) ([]*TelegramSettings, error) {
	var list []*TelegramSettings
	err := s.db.Where("user_id = ?", userID).Find(&list).Error
	return list, err
}

func (s *TelegramStore) GetByTraderID(traderID string) (*TelegramSettings, error) {
	var settings TelegramSettings
	err := s.db.Where("trader_id = ?", traderID).First(&settings).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &settings, err
}

func (s *TelegramStore) GetDefaultTrader(userID string) (*TelegramSettings, error) {
	var settings TelegramSettings
	err := s.db.Where("user_id = ? AND default_trader = ?", userID, true).First(&settings).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &settings, err
}

func (s *TelegramStore) SetDefaultTrader(userID, traderID string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&TelegramSettings{}).
			Where("user_id = ?", userID).
			Update("default_trader", false).Error; err != nil {
			return err
		}
		return tx.Model(&TelegramSettings{}).
			Where("user_id = ? AND trader_id = ?", userID, traderID).
			Update("default_trader", true).Error
	})
}

func (s *TelegramStore) GetEnabledSettings() ([]*TelegramSettings, error) {
	var list []*TelegramSettings
	err := s.db.Where("enabled = ?", true).Find(&list).Error
	return list, err
}

func (s *TelegramStore) Delete(userID, traderID string) error {
	return s.db.Where("user_id = ? AND trader_id = ?", userID, traderID).Delete(&TelegramSettings{}).Error
}
