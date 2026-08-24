package channel

import (
	"errors"
	"strings"
	"sync"
)

var (
	ErrInvalidChannel    = errors.New("invalid session channel")
	ErrAlreadyRegistered = errors.New("session channel already registered")
)

type Registry struct {
	mu       sync.RWMutex
	channels map[string]*Channel
}

func NewRegistry() *Registry {
	return &Registry{channels: make(map[string]*Channel)}
}

func (r *Registry) Register(sessionChannel *Channel) error {
	if sessionChannel == nil || sessionChannel.controller == nil || strings.TrimSpace(sessionChannel.sessionID) == "" {
		return ErrInvalidChannel
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.channels[sessionChannel.sessionID]; exists {
		return ErrAlreadyRegistered
	}
	r.channels[sessionChannel.sessionID] = sessionChannel
	return nil
}

func (r *Registry) Get(sessionID string) (*Channel, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	sessionChannel, exists := r.channels[sessionID]
	return sessionChannel, exists
}

func (r *Registry) Unregister(sessionID string) {
	r.mu.Lock()
	delete(r.channels, sessionID)
	r.mu.Unlock()
}
