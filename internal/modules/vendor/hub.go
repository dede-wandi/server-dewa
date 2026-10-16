package vendor

import (
	"sync"
)

// InquiryEvent holds data received from an asynchronous inquiry callback (e.g. from DFlash OtomaX)
type InquiryEvent struct {
	RefID   string
	Status  string // SUCCESS, FAILED
	Message string
	SN      string
	Price   float64
}

var (
	inquiryWaiters = make(map[string]chan *InquiryEvent)
	inquiryMu      sync.RWMutex
)

// RegisterInquiryWaiter creates a channel to wait for an asynchronous inquiry result
func RegisterInquiryWaiter(refID string) chan *InquiryEvent {
	inquiryMu.Lock()
	defer inquiryMu.Unlock()

	ch := make(chan *InquiryEvent, 1)
	inquiryWaiters[refID] = ch
	return ch
}

// NotifyInquiryWaiter notifies any waiting inquiry request and cleans up
func NotifyInquiryWaiter(refID string, ev *InquiryEvent) bool {
	inquiryMu.Lock()
	defer inquiryMu.Unlock()

	ch, exists := inquiryWaiters[refID]
	if !exists {
		return false
	}

	select {
	case ch <- ev:
	default:
	}
	delete(inquiryWaiters, refID)
	return true
}

// RemoveInquiryWaiter removes a waiter channel if it times out
func RemoveInquiryWaiter(refID string) {
	inquiryMu.Lock()
	defer inquiryMu.Unlock()
	delete(inquiryWaiters, refID)
}
