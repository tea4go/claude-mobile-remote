package main

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/tea4go/claude-mobile-remote/internal/channel"
	"github.com/tea4go/claude-mobile-remote/internal/pairing"
)

type transportServer interface {
	Start() error
	BeginShutdown()
	Close(context.Context) error
}

type remoteRuntime struct {
	manager    *pairing.Manager
	registry   *channel.Registry
	server     transportServer
	controller channel.Controller

	mu          sync.Mutex
	started     bool
	rotationErr error
	ticks       <-chan time.Time
	ticker      *time.Ticker
	pairings    chan pairing.Payload
	stop        chan struct{}
	done        chan struct{}
	shutdown    sync.Once
	doneOnce    sync.Once
}

func newRemoteRuntime(manager *pairing.Manager, registry *channel.Registry, server transportServer, controller channel.Controller) *remoteRuntime {
	return &remoteRuntime{
		manager:    manager,
		registry:   registry,
		server:     server,
		controller: controller,
		pairings:   make(chan pairing.Payload, 1),
		stop:       make(chan struct{}),
		done:       make(chan struct{}),
	}
}

func (r *remoteRuntime) Start() error {
	if err := r.server.Start(); err != nil {
		return err
	}
	r.mu.Lock()
	if r.ticks == nil {
		r.ticker = time.NewTicker(time.Second)
		r.ticks = r.ticker.C
	}
	r.started = true
	r.mu.Unlock()
	go r.rotateLoop()
	return nil
}

func (r *remoteRuntime) Activate(sessionID string) (pairing.Payload, error) {
	sessionChannel := channel.New(sessionID, r.controller)
	if err := r.registry.Register(sessionChannel); err != nil {
		return pairing.Payload{}, err
	}
	payload, err := r.manager.Issue(sessionID)
	if err != nil {
		r.registry.Unregister(sessionID)
		return pairing.Payload{}, err
	}
	return payload, nil
}

func (r *remoteRuntime) Pairings() <-chan pairing.Payload {
	return r.pairings
}

func (r *remoteRuntime) BeginShutdown() {
	r.shutdown.Do(func() {
		r.manager.Invalidate()
		r.server.BeginShutdown()
		close(r.stop)
		r.mu.Lock()
		started := r.started
		r.mu.Unlock()
		if !started {
			r.doneOnce.Do(func() { close(r.done) })
		}
	})
}

func (r *remoteRuntime) Close(ctx context.Context) error {
	r.BeginShutdown()
	var waitErr error
	select {
	case <-r.done:
	case <-ctx.Done():
		waitErr = ctx.Err()
	}
	serverErr := r.server.Close(ctx)
	r.manager.Revoke()
	r.mu.Lock()
	rotationErr := r.rotationErr
	r.mu.Unlock()
	return errors.Join(waitErr, serverErr, rotationErr)
}

func (r *remoteRuntime) rotateLoop() {
	defer r.doneOnce.Do(func() { close(r.done) })
	defer func() {
		if r.ticker != nil {
			r.ticker.Stop()
		}
	}()
	for {
		select {
		case <-r.stop:
			return
		case <-r.ticks:
			payload, rotated, err := r.manager.RotateIfExpired()
			if errors.Is(err, pairing.ErrNoCredential) {
				continue
			}
			if err != nil {
				r.mu.Lock()
				r.rotationErr = err
				r.mu.Unlock()
				continue
			}
			if rotated {
				r.publishLatest(payload)
			}
		}
	}
}

func (r *remoteRuntime) publishLatest(payload pairing.Payload) {
	select {
	case r.pairings <- payload:
		return
	default:
	}
	select {
	case <-r.pairings:
	default:
	}
	select {
	case r.pairings <- payload:
	case <-r.stop:
	}
}
