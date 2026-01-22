//go:build cgo

package trader

import (
	"sync"
	"time"
)

type mockTrader struct {
	mu sync.Mutex

	openLongCalls   int
	openShortCalls  int
	closeLongCalls  int
	closeShortCalls int

	openLongResult   map[string]interface{}
	openShortResult  map[string]interface{}
	closeLongResult  map[string]interface{}
	closeShortResult map[string]interface{}

	openLongErr   error
	openShortErr  error
	closeLongErr  error
	closeShortErr error
}

func (m *mockTrader) calls() (openLong, openShort, closeLong, closeShort int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.openLongCalls, m.openShortCalls, m.closeLongCalls, m.closeShortCalls
}

func (m *mockTrader) GetBalance() (map[string]interface{}, error) {
	return nil, nil
}

func (m *mockTrader) GetPositions() ([]map[string]interface{}, error) {
	return nil, nil
}

func (m *mockTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	m.mu.Lock()
	m.openLongCalls++
	res := m.openLongResult
	err := m.openLongErr
	m.mu.Unlock()
	return res, err
}

func (m *mockTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	m.mu.Lock()
	m.openShortCalls++
	res := m.openShortResult
	err := m.openShortErr
	m.mu.Unlock()
	return res, err
}

func (m *mockTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	m.mu.Lock()
	m.closeLongCalls++
	res := m.closeLongResult
	err := m.closeLongErr
	m.mu.Unlock()
	return res, err
}

func (m *mockTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	m.mu.Lock()
	m.closeShortCalls++
	res := m.closeShortResult
	err := m.closeShortErr
	m.mu.Unlock()
	return res, err
}

func (m *mockTrader) SetLeverage(symbol string, leverage int) error {
	return nil
}

func (m *mockTrader) SetMarginMode(symbol string, isCrossMargin bool) error {
	return nil
}

func (m *mockTrader) GetMarketPrice(symbol string) (float64, error) {
	return 0, nil
}

func (m *mockTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	return nil
}

func (m *mockTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	return nil
}

func (m *mockTrader) CancelStopLossOrders(symbol string) error {
	return nil
}

func (m *mockTrader) CancelTakeProfitOrders(symbol string) error {
	return nil
}

func (m *mockTrader) CancelAllOrders(symbol string) error {
	return nil
}

func (m *mockTrader) CancelStopOrders(symbol string) error {
	return nil
}

func (m *mockTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	return "", nil
}

func (m *mockTrader) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) {
	return nil, nil
}

func (m *mockTrader) GetClosedPnL(startTime time.Time, limit int) ([]ClosedPnLRecord, error) {
	return nil, nil
}

func (m *mockTrader) GetOpenOrders(symbol string) ([]OpenOrder, error) {
	return nil, nil
}
