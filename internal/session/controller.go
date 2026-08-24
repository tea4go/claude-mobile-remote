package session

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	projectclaude "github.com/tea4go/claude-mobile-remote/internal/claude"
)

// Option configures a Controller before it starts.
type Option func(*Controller)

func withRequestIDGenerator(generator requestIDGenerator) Option {
	return func(controller *Controller) {
		if generator != nil {
			controller.requestIDGenerator = generator
		}
	}
}

// Controller serializes all Session state and Host work through one actor.
type Controller struct {
	worker             *hostWorker
	commands           chan actorCommand
	startResult        chan error
	actorDone          chan struct{}
	initialSnapshot    Snapshot
	requestIDGenerator requestIDGenerator

	startMu      sync.Mutex
	startCalled  bool
	actorLive    atomic.Bool
	workerCancel context.CancelFunc
	workerStop   sync.Once
	actorClose   sync.Once

	finalMu            sync.Mutex
	finalSnapshot      Snapshot
	finalSnapshotReady bool
	finalStopErr       error
}

type queuedInput struct {
	RequestID string
	Input     Input
}

type actorSubscriber struct {
	events    chan Event
	cancelled <-chan struct{}
}

type actorState struct {
	snapshot             Snapshot
	queue                []queuedInput
	current              *queuedInput
	currentCancel        context.CancelFunc
	currentTerminal      bool
	turnStartedPublished bool
	preSessionEvents     []workerEvent
	subscribers          map[uint64]actorSubscriber
	nextSubscriberID     uint64
	connected            bool
	startResolved        bool
	stopping             bool
	workerClosed         bool
	workerCloseErr       error
	pendingStops         []chan error
	pendingIdle          []chan snapshotResponse
}

type actorCommand interface {
	isActorCommand()
}

type submitCommand struct {
	input    Input
	response chan submitResponse
}

type submitResponse struct {
	requestID string
	err       error
}

func (submitCommand) isActorCommand() {}

type snapshotCommand struct {
	response chan snapshotResponse
}

type snapshotResponse struct {
	snapshot Snapshot
	err      error
}

func (snapshotCommand) isActorCommand() {}

type waitIdleCommand struct {
	response chan snapshotResponse
}

func (waitIdleCommand) isActorCommand() {}

type subscribeCommand struct {
	ctx      context.Context
	capacity int
	response chan subscribeResponse
}

type subscribeResponse struct {
	snapshot     Snapshot
	events       <-chan Event
	subscriberID uint64
	err          error
}

func (subscribeCommand) isActorCommand() {}

type unsubscribeCommand struct {
	subscriberID uint64
}

func (unsubscribeCommand) isActorCommand() {}

type stopCommand struct {
	response chan error
}

func (stopCommand) isActorCommand() {}

// NewController constructs a not-yet-started Controller with its initial state.
func NewController(host Host, options ...Option) *Controller {
	controller := &Controller{
		worker:      newHostWorker(host),
		commands:    make(chan actorCommand),
		startResult: make(chan error, 1),
		actorDone:   make(chan struct{}),
		initialSnapshot: Snapshot{
			SessionStatus: SessionStarting,
			Turn: TurnSnapshot{
				Status: TurnIdle,
			},
		},
		requestIDGenerator: randomRequestID,
	}
	for _, option := range options {
		option(controller)
	}
	return controller
}

// Start launches the actor and the sole Host worker, then waits for Connect.
func (c *Controller) Start(ctx context.Context) error {
	c.startMu.Lock()
	if c.startCalled {
		c.startMu.Unlock()
		return ErrSessionUnavailable
	}
	c.startCalled = true
	workerContext, workerCancel := context.WithCancel(context.Background())
	c.workerCancel = workerCancel
	c.actorLive.Store(true)
	go c.runActor()
	c.worker.start(workerContext)
	go c.stopWhenServiceContextEnds(ctx)
	c.startMu.Unlock()

	select {
	case err := <-c.startResult:
		if contextErr := ctx.Err(); contextErr != nil {
			return contextErr
		}
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Submit accepts an input into the actor's FIFO. Once delivered to the actor,
// the acceptance handshake is completed even if the caller context is canceled.
func (c *Controller) Submit(ctx context.Context, input Input) (string, error) {
	if !c.actorLive.Load() {
		return "", ErrSessionUnavailable
	}
	response := make(chan submitResponse)
	command := submitCommand{input: input, response: response}
	select {
	case c.commands <- command:
		result := <-response
		return result.requestID, result.err
	case <-ctx.Done():
		return "", ctx.Err()
	case <-c.actorDone:
		return "", ErrSessionUnavailable
	}
}

// Snapshot returns an actor-consistent copy of the current Session state.
func (c *Controller) Snapshot(ctx context.Context) (Snapshot, error) {
	if !c.actorLive.Load() {
		if snapshot, ok := c.cachedFinalSnapshot(); ok {
			return snapshot, nil
		}
		return Snapshot{}, ErrSessionUnavailable
	}
	response := make(chan snapshotResponse, 1)
	command := snapshotCommand{response: response}
	select {
	case c.commands <- command:
	case <-ctx.Done():
		return Snapshot{}, ctx.Err()
	case <-c.actorDone:
		if snapshot, ok := c.cachedFinalSnapshot(); ok {
			return snapshot, nil
		}
		return Snapshot{}, ErrSessionUnavailable
	}

	select {
	case result := <-response:
		return result.snapshot, result.err
	case <-ctx.Done():
		return Snapshot{}, ctx.Err()
	case <-c.actorDone:
		if snapshot, ok := c.cachedFinalSnapshot(); ok {
			return snapshot, nil
		}
		return Snapshot{}, ErrSessionUnavailable
	}
}

// WaitIdle waits until the actor has processed the final Worker result for all
// accepted work and returns an actor-consistent Snapshot.
func (c *Controller) WaitIdle(ctx context.Context) (Snapshot, error) {
	if !c.actorLive.Load() {
		if snapshot, ok := c.cachedFinalSnapshot(); ok {
			return snapshot, nil
		}
		return Snapshot{}, ErrSessionUnavailable
	}
	response := make(chan snapshotResponse, 1)
	command := waitIdleCommand{response: response}
	select {
	case c.commands <- command:
	case <-ctx.Done():
		return Snapshot{}, ctx.Err()
	case <-c.actorDone:
		if snapshot, ok := c.cachedFinalSnapshot(); ok {
			return snapshot, nil
		}
		return Snapshot{}, ErrSessionUnavailable
	}

	select {
	case result := <-response:
		return result.snapshot, result.err
	case <-ctx.Done():
		return Snapshot{}, ctx.Err()
	case <-c.actorDone:
		if snapshot, ok := c.cachedFinalSnapshot(); ok {
			return snapshot, nil
		}
		return Snapshot{}, ErrSessionUnavailable
	}
}

// Subscribe atomically registers a trusted in-process consumer and captures
// the snapshot at the same actor boundary.
func (c *Controller) Subscribe(ctx context.Context, capacity int) (Snapshot, <-chan Event, error) {
	if capacity <= 0 {
		return Snapshot{}, nil, ErrInvalidInput
	}
	if !c.actorLive.Load() {
		return Snapshot{}, nil, ErrSessionUnavailable
	}
	response := make(chan subscribeResponse, 1)
	command := subscribeCommand{ctx: ctx, capacity: capacity, response: response}
	select {
	case c.commands <- command:
	case <-ctx.Done():
		return Snapshot{}, nil, ctx.Err()
	case <-c.actorDone:
		return Snapshot{}, nil, ErrSessionUnavailable
	}

	var result subscribeResponse
	select {
	case result = <-response:
	case <-ctx.Done():
		return Snapshot{}, nil, ctx.Err()
	case <-c.actorDone:
		return Snapshot{}, nil, ErrSessionUnavailable
	}
	if result.err != nil {
		return Snapshot{}, nil, result.err
	}
	return result.snapshot, result.events, nil
}

// Stop starts deterministic worker shutdown and waits for the actor to finish.
func (c *Controller) Stop(ctx context.Context) error {
	if !c.actorLive.Load() {
		if err, ok := c.cachedStopResult(); ok {
			return err
		}
		return ErrSessionUnavailable
	}
	response := make(chan error, 1)
	command := stopCommand{response: response}
	select {
	case c.commands <- command:
	case <-c.actorDone:
		if err, ok := c.cachedStopResult(); ok {
			return err
		}
		return ErrSessionUnavailable
	case <-ctx.Done():
		return ctx.Err()
	}

	select {
	case err := <-response:
		return err
	case <-c.actorDone:
		if err, ok := c.cachedStopResult(); ok {
			return err
		}
		return ErrSessionUnavailable
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *Controller) stopWhenServiceContextEnds(ctx context.Context) {
	select {
	case <-ctx.Done():
		_ = c.Stop(context.Background())
	case <-c.actorDone:
	}
}

func (c *Controller) unsubscribeWhenContextEnds(ctx context.Context, subscriberID uint64) {
	select {
	case <-ctx.Done():
		select {
		case c.commands <- unsubscribeCommand{subscriberID: subscriberID}:
		case <-c.actorDone:
		}
	case <-c.actorDone:
	}
}

func (c *Controller) runActor() {
	state := actorState{
		snapshot:    cloneSnapshot(c.initialSnapshot),
		subscribers: make(map[uint64]actorSubscriber),
	}

	for {
		select {
		case command := <-c.commands:
			if c.handleCommand(&state, command) {
				return
			}
		case connectErr := <-c.worker.connected:
			c.handleConnectResult(&state, connectErr)
			if state.stopping && state.workerClosed {
				c.finishStop(&state)
				return
			}
		case event := <-c.worker.events:
			c.handleWorkerEvent(&state, event)
		case result := <-c.worker.results:
			c.handleWorkerResult(&state, result)
		case closeErr := <-c.worker.closed:
			state.workerClosed = true
			state.workerCloseErr = closeErr
			if state.stopping {
				c.finishStop(&state)
				return
			}
		}
	}
}

func (c *Controller) handleCommand(state *actorState, command actorCommand) bool {
	switch command := command.(type) {
	case submitCommand:
		c.handleSubmit(state, command)
	case snapshotCommand:
		command.response <- snapshotResponse{snapshot: cloneSnapshot(state.snapshot)}
	case waitIdleCommand:
		if !state.stopping && state.current == nil && len(state.queue) == 0 {
			command.response <- snapshotResponse{snapshot: cloneSnapshot(state.snapshot)}
		} else {
			state.pendingIdle = append(state.pendingIdle, command.response)
		}
	case subscribeCommand:
		state.nextSubscriberID++
		subscriberID := state.nextSubscriberID
		events := make(chan Event, command.capacity)
		state.subscribers[subscriberID] = actorSubscriber{
			events:    events,
			cancelled: command.ctx.Done(),
		}
		go c.unsubscribeWhenContextEnds(command.ctx, subscriberID)
		command.response <- subscribeResponse{
			snapshot:     cloneSnapshot(state.snapshot),
			events:       events,
			subscriberID: subscriberID,
		}
	case unsubscribeCommand:
		c.removeSubscriber(state, command.subscriberID)
	case stopCommand:
		state.pendingStops = append(state.pendingStops, command.response)
		if !state.stopping {
			c.beginStop(state)
		}
		if state.workerClosed {
			c.finishStop(state)
			return true
		}
	}
	return false
}

func (c *Controller) handleSubmit(state *actorState, command submitCommand) {
	if !state.connected || state.stopping || state.snapshot.SessionStatus == SessionStopped || state.snapshot.SessionStatus == SessionFailed {
		command.response <- submitResponse{err: ErrSessionUnavailable}
		return
	}

	input, err := normalizeInput(state.snapshot.SessionID, command.input)
	if err != nil {
		command.response <- submitResponse{err: err}
		return
	}
	if state.current != nil && len(state.queue) >= QueueCapacity {
		command.response <- submitResponse{err: ErrQueueFull}
		return
	}

	requestID, err := c.requestIDGenerator()
	if err != nil {
		if !errors.Is(err, ErrInternal) {
			err = fmt.Errorf("%w: generate request ID: %v", ErrInternal, err)
		}
		command.response <- submitResponse{err: err}
		return
	}
	state.queue = append(state.queue, queuedInput{RequestID: requestID, Input: input})
	state.snapshot.QueueDepth = len(state.queue)

	// This synchronous unbuffered handoff delivers the acceptance result to the
	// Submit goroutine before startNext. It does not promise that caller code
	// after Submit's return is scheduled before the actor continues; A4 owns
	// externally observable command-result/event ordering.
	command.response <- submitResponse{requestID: requestID}
	c.startNext(state)
}

func (c *Controller) startNext(state *actorState) {
	if state.current != nil || len(state.queue) == 0 || state.stopping || state.snapshot.SessionStatus == SessionStopped || state.snapshot.SessionStatus == SessionFailed {
		return
	}

	next := state.queue[0]
	state.queue[0] = queuedInput{}
	state.queue = state.queue[1:]
	state.snapshot.QueueDepth = len(state.queue)
	state.current = &next
	turnContext, cancel := context.WithCancel(context.Background())
	state.currentCancel = cancel
	state.currentTerminal = false
	state.turnStartedPublished = false
	state.snapshot.Turn = TurnSnapshot{
		Status:        TurnRunning,
		RequestID:     next.RequestID,
		Source:        next.Input.Source,
		PendingAction: nil,
	}

	if state.snapshot.SessionID != "" {
		state.turnStartedPublished = true
		c.publish(state, Event{
			Kind:       EventTurnStarted,
			SessionID:  state.snapshot.SessionID,
			RequestID:  next.RequestID,
			Source:     next.Input.Source,
			TurnStatus: TurnRunning,
		})
	}

	request := workerRequest{
		RequestID: next.RequestID,
		Prompt:    next.Input.Text,
		Context:   turnContext,
	}
	go func() {
		select {
		case c.worker.requests <- request:
		case <-c.actorDone:
		}
	}()
}

func (c *Controller) handleConnectResult(state *actorState, connectErr error) {
	if connectErr == nil {
		if state.stopping {
			c.completeStart(state, ErrSessionUnavailable)
			return
		}
		state.connected = true
		c.completeStart(state, nil)
		return
	}
	if state.stopping {
		if !state.workerClosed {
			state.workerCloseErr = <-c.worker.closed
			state.workerClosed = true
		}
		c.completeStart(state, ErrSessionUnavailable)
		return
	}

	c.failSession(state, connectErr)
	if !state.workerClosed {
		state.workerCloseErr = <-c.worker.closed
		state.workerClosed = true
	}
	c.completeStart(state, connectErr)
}

func (c *Controller) completeStart(state *actorState, err error) {
	if state.startResolved {
		return
	}
	state.startResolved = true
	c.startResult <- err
}

func (c *Controller) handleWorkerEvent(state *actorState, event workerEvent) {
	if state.stopping {
		return
	}
	if state.current == nil || state.current.RequestID != event.RequestID || state.currentTerminal {
		return
	}

	switch event.Adapter.Kind {
	case projectclaude.EventSessionStarted:
		if state.snapshot.SessionID != "" {
			return
		}
		state.snapshot.SessionID = event.Adapter.SessionID
		state.snapshot.SessionStatus = SessionReady
		state.snapshot.SessionError = nil
		c.publish(state, Event{
			Kind:          EventSessionStarted,
			SessionID:     state.snapshot.SessionID,
			SessionStatus: SessionReady,
		})
		if !state.turnStartedPublished {
			state.turnStartedPublished = true
			c.publish(state, Event{
				Kind:       EventTurnStarted,
				SessionID:  state.snapshot.SessionID,
				RequestID:  state.current.RequestID,
				Source:     state.current.Input.Source,
				TurnStatus: TurnRunning,
			})
		}
		buffered := state.preSessionEvents
		state.preSessionEvents = nil
		for _, bufferedEvent := range buffered {
			c.appendText(state, bufferedEvent)
		}
	case projectclaude.EventText:
		if state.snapshot.SessionID == "" {
			state.preSessionEvents = append(state.preSessionEvents, event)
			return
		}
		c.appendText(state, event)
	case projectclaude.EventTurnCompleted:
		state.snapshot.Turn.Status = TurnCompleted
		state.snapshot.Turn.Error = nil
		state.snapshot.Turn.PendingAction = nil
		state.currentTerminal = true
		state.preSessionEvents = nil
		c.publish(state, Event{
			Kind:       EventTurnCompleted,
			SessionID:  state.snapshot.SessionID,
			RequestID:  state.current.RequestID,
			Source:     state.current.Input.Source,
			TurnStatus: TurnCompleted,
		})
	case projectclaude.EventTurnFailed:
		message := event.Adapter.Text
		if message == "" {
			message = "Claude Turn failed"
		}
		c.recordTurnFailure(state, message)
	}
}

func (c *Controller) handleWorkerResult(state *actorState, result workerResult) {
	if state.current == nil || state.current.RequestID != result.RequestID {
		return
	}

	if !state.stopping {
		switch {
		case result.Err == nil && !state.currentTerminal:
			c.failSession(state, errors.New("Host returned without a terminal event"))
		case result.Err != nil && projectclaude.ScopeOf(result.Err) == projectclaude.ScopeSession:
			c.failSession(state, result.Err)
		case result.Err != nil && !state.currentTerminal:
			c.failTurn(state, result.Err)
		}
	}

	if state.currentCancel != nil {
		state.currentCancel()
	}
	state.currentCancel = nil
	state.current = nil
	state.currentTerminal = false
	state.turnStartedPublished = false
	state.preSessionEvents = nil
	state.snapshot.Turn.PendingAction = nil
	if state.stopping {
		c.stopWorker()
	} else if state.snapshot.SessionStatus == SessionStarting || state.snapshot.SessionStatus == SessionReady {
		c.startNext(state)
	}
	if !state.stopping {
		c.resolveIdleWaiters(state)
	}
}

func (c *Controller) appendText(state *actorState, event workerEvent) {
	if state.current == nil || state.current.RequestID != event.RequestID || state.currentTerminal {
		return
	}
	state.snapshot.Turn.Message += event.Adapter.Text
	c.publish(state, Event{
		Kind:      EventTextDelta,
		SessionID: state.snapshot.SessionID,
		RequestID: state.current.RequestID,
		Source:    state.current.Input.Source,
		Text:      event.Adapter.Text,
	})
}

func (c *Controller) failTurn(state *actorState, err error) {
	c.recordTurnFailure(state, err.Error())
}

func (c *Controller) recordTurnFailure(state *actorState, message string) {
	if state.current == nil || state.currentTerminal {
		return
	}
	errorInfo := &ErrorInfo{Code: "turn_failed", Message: message}
	state.snapshot.Turn.Status = TurnFailed
	state.snapshot.Turn.Error = errorInfo
	state.snapshot.Turn.PendingAction = nil
	state.currentTerminal = true
	state.preSessionEvents = nil
	c.publish(state, Event{
		Kind:       EventTurnFailed,
		SessionID:  state.snapshot.SessionID,
		RequestID:  state.current.RequestID,
		Source:     state.current.Input.Source,
		TurnStatus: TurnFailed,
		Error:      cloneErrorInfo(errorInfo),
	})
}

func (c *Controller) failSession(state *actorState, err error) {
	if state.snapshot.SessionStatus == SessionFailed {
		return
	}
	if state.current != nil && !state.currentTerminal {
		c.failTurn(state, err)
	}
	state.snapshot.SessionStatus = SessionFailed
	state.snapshot.SessionError = &ErrorInfo{Code: "session_failed", Message: err.Error()}
	state.queue = nil
	state.snapshot.QueueDepth = 0
	state.preSessionEvents = nil
	if state.currentCancel != nil {
		state.currentCancel()
	}
	c.publish(state, Event{
		Kind:          EventSessionFailed,
		SessionID:     state.snapshot.SessionID,
		SessionStatus: SessionFailed,
		Error:         cloneErrorInfo(state.snapshot.SessionError),
	})
}

func (c *Controller) publish(state *actorState, event Event) {
	state.snapshot.Seq++
	event.Seq = state.snapshot.Seq
	for subscriberID, subscriber := range state.subscribers {
		published := event
		published.Error = cloneErrorInfo(event.Error)
		select {
		case <-subscriber.cancelled:
			c.removeSubscriber(state, subscriberID)
			continue
		default:
		}
		select {
		case subscriber.events <- published:
		case <-subscriber.cancelled:
			c.removeSubscriber(state, subscriberID)
		}
	}
}

func (c *Controller) removeSubscriber(state *actorState, subscriberID uint64) {
	if subscriber, exists := state.subscribers[subscriberID]; exists {
		delete(state.subscribers, subscriberID)
		close(subscriber.events)
	}
}

func cloneSnapshot(snapshot Snapshot) Snapshot {
	cloned := snapshot
	cloned.SessionError = cloneErrorInfo(snapshot.SessionError)
	cloned.Turn.Error = cloneErrorInfo(snapshot.Turn.Error)
	if snapshot.Turn.PendingAction != nil {
		pendingAction := *snapshot.Turn.PendingAction
		cloned.Turn.PendingAction = &pendingAction
	}
	return cloned
}

func cloneErrorInfo(errorInfo *ErrorInfo) *ErrorInfo {
	if errorInfo == nil {
		return nil
	}
	cloned := *errorInfo
	return &cloned
}

func (c *Controller) beginStop(state *actorState) {
	state.stopping = true
	state.queue = nil
	state.snapshot.QueueDepth = 0
	state.preSessionEvents = nil
	state.snapshot.Turn.PendingAction = nil
	if state.current != nil {
		if !state.currentTerminal {
			state.snapshot.Turn.Status = TurnCancelled
			state.snapshot.Turn.Error = nil
		}
		if state.currentCancel != nil {
			state.currentCancel()
		}
	}
	if c.workerCancel != nil {
		c.workerCancel()
	}
	if state.current == nil {
		c.stopWorker()
	}
}

func (c *Controller) finishStop(state *actorState) {
	c.completeStart(state, ErrSessionUnavailable)
	if state.snapshot.SessionStatus != SessionFailed {
		state.snapshot.SessionStatus = SessionStopped
		state.snapshot.SessionError = nil
		c.publish(state, Event{
			Kind:          EventSessionStopped,
			SessionID:     state.snapshot.SessionID,
			SessionStatus: SessionStopped,
		})
	}
	state.current = nil
	state.currentCancel = nil
	state.preSessionEvents = nil
	c.cacheFinal(state.snapshot, state.workerCloseErr)
	c.resolveIdleWaiters(state)
	for subscriberID, subscriber := range state.subscribers {
		delete(state.subscribers, subscriberID)
		close(subscriber.events)
	}
	for _, response := range state.pendingStops {
		response <- state.workerCloseErr
	}
	state.pendingStops = nil
	c.actorLive.Store(false)
	c.actorClose.Do(func() {
		close(c.actorDone)
	})
}

func (c *Controller) resolveIdleWaiters(state *actorState) {
	if state.current != nil || len(state.queue) != 0 {
		return
	}
	for _, response := range state.pendingIdle {
		response <- snapshotResponse{snapshot: cloneSnapshot(state.snapshot)}
	}
	state.pendingIdle = nil
}

func (c *Controller) stopWorker() {
	c.workerStop.Do(func() {
		close(c.worker.stop)
	})
}

func (c *Controller) cacheFinal(snapshot Snapshot, err error) {
	c.finalMu.Lock()
	c.finalSnapshot = cloneSnapshot(snapshot)
	c.finalSnapshotReady = true
	c.finalStopErr = err
	c.finalMu.Unlock()
}

func (c *Controller) cachedFinalSnapshot() (Snapshot, bool) {
	c.finalMu.Lock()
	defer c.finalMu.Unlock()
	if !c.finalSnapshotReady {
		return Snapshot{}, false
	}
	return cloneSnapshot(c.finalSnapshot), true
}

func (c *Controller) cachedStopResult() (error, bool) {
	c.finalMu.Lock()
	defer c.finalMu.Unlock()
	if !c.finalSnapshotReady {
		return nil, false
	}
	return c.finalStopErr, true
}
