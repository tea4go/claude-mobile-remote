package session

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	projectclaude "github.com/tea4go/claude-mobile-remote/internal/claude"
	"github.com/tea4go/claude-mobile-remote/protocol"
)

// The interaction control vocabulary lives in protocol, the single Go source of
// truth for the wire contract. The sentinels and the ActionDecision/QuestionAnswer
// types are aliased here so existing Session callers keep compiling.
var (
	ErrActionNotPending   = protocol.ErrActionNotPending
	ErrInvalidAction      = protocol.ErrInvalidAction
	ErrInvalidAnswer      = protocol.ErrInvalidAnswer
	ErrTurnNotCancellable = protocol.ErrTurnNotCancellable
	ErrRequestNotQueued   = protocol.ErrRequestNotQueued
)

type (
	ActionDecision = protocol.ActionDecision
	QuestionAnswer = protocol.QuestionAnswer
)

const (
	DecisionAllowOnce    = protocol.DecisionAllowOnce
	DecisionAllowSession = protocol.DecisionAllowSession
	DecisionDeny         = protocol.DecisionDeny
)

// The deny reasons Claude sees. They explain the outcome without echoing user
// content, tool input or client identities.
const (
	denyReasonNoTurn       = "no active Turn can accept this action"
	denyReasonConcurrent   = "another action is already awaiting a decision"
	denyReasonTextDisabled = "free-text questions are disabled for this session"
	denyReasonUnrenderable = "the question could not be presented safely"
	denyReasonUser         = "the user rejected this action"
	denyReasonCancelled    = "the Turn was cancelled"
	denyReasonTerminated   = "the Turn ended before a decision arrived"
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

// WithReservedSessionID 预留真实 Claude session ID（A5.2 §4）：ID 从创建起
// 不可变并进入 snapshot，但 Claude init 确认前 Session 保持 starting。
func WithReservedSessionID(sessionID string) Option {
	return func(controller *Controller) {
		if sessionID != "" {
			controller.initialSnapshot.SessionID = sessionID
		}
	}
}

// WithDisplayMetadata supplies the fixed working directory and, for new
// sessions only, their actual creation time. Resume must pass a zero time.
func WithDisplayMetadata(cwd string, createdAt time.Time) Option {
	return func(controller *Controller) {
		controller.initialSnapshot.ProjectCwd = cwd
		if !createdAt.IsZero() {
			controller.initialSnapshot.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
		}
	}
}

// WithServiceConfig overrides the service knobs that shape published state, so a
// caller can raise the preview limit or opt into free-text questions without any
// other component holding its own copy of those defaults (B2 §4.3).
func WithServiceConfig(config ServiceConfig) Option {
	return func(controller *Controller) {
		if config.ToolPreviewMaxRunes > 0 {
			controller.config.ToolPreviewMaxRunes = config.ToolPreviewMaxRunes
		}
		controller.config.AskUserTextEnabled = config.AskUserTextEnabled
		if config.PermissionMode.IsKnown() {
			controller.config.PermissionMode = config.PermissionMode
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
	config             ServiceConfig

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
	// sessionConfirmed 报告 Claude init/Result 是否已确认 session ID
	//（A5.2 §4）。预留 ID 只影响路由与配对，不影响该状态。
	sessionConfirmed bool
	// pendingAction is the one blocked permission callback, if any. It is kept
	// beside the published snapshot copy so the actor can always release the SDK
	// read loop, which stays stuck until the callback returns (B2 §3.4).
	pendingAction   *projectclaude.ActionRequest
	pendingActionID string
	nextActionSeq   uint64
	// nextBlockSeq numbers the client-facing content block IDs. Like nextActionSeq
	// it stays monotonic across Turns, so an ID can never be reused.
	nextBlockSeq uint64
	// currentCancelled marks a Turn the Controller cancelled itself. The Host
	// then reports context.Canceled, which is Session-scoped by default and would
	// otherwise fail the whole Session.
	currentCancelled bool
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

type resolveActionCommand struct {
	actionID string
	decision ActionDecision
	answers  []QuestionAnswer
	response chan error
}

func (resolveActionCommand) isActorCommand() {}

type cancelTurnCommand struct {
	requestID string
	response  chan error
}

func (cancelTurnCommand) isActorCommand() {}

type cancelQueuedCommand struct {
	requestID string
	response  chan error
}

func (cancelQueuedCommand) isActorCommand() {}

type setPermissionModeCommand struct {
	mode     protocol.PermissionMode
	response chan error
}

func (setPermissionModeCommand) isActorCommand() {}

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
		config:             DefaultServiceConfig(),
	}
	for _, option := range options {
		option(controller)
	}
	controller.initialSnapshot.PermissionMode = controller.config.PermissionMode
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

// ReservedSessionID 返回预留的真实 session ID（A5.2）；未预留时为空串，
// 上层沿用 init 后激活的旧路径。
func (c *Controller) ReservedSessionID() string {
	return c.initialSnapshot.SessionID
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

// ResolveAction applies one client decision to the pending action. The decision
// always travels through the actor before it reaches the SDK, so Transport can
// never wake a blocked permission callback directly (B2 §3.2).
func (c *Controller) ResolveAction(ctx context.Context, actionID string, decision ActionDecision, answers []QuestionAnswer) error {
	return c.sendControl(ctx, func(response chan error) actorCommand {
		return resolveActionCommand{actionID: actionID, decision: decision, answers: answers, response: response}
	})
}

// CancelTurn cancels the running Turn. The Session stays usable: only a stop or
// a Session-level failure refuses further work.
func (c *Controller) CancelTurn(ctx context.Context, requestID string) error {
	return c.sendControl(ctx, func(response chan error) actorCommand {
		return cancelTurnCommand{requestID: requestID, response: response}
	})
}

// CancelQueued drops one not-yet-started FIFO entry. It never touches the
// running Turn or the other queued inputs (B2 §3.2).
func (c *Controller) CancelQueued(ctx context.Context, requestID string) error {
	return c.sendControl(ctx, func(response chan error) actorCommand {
		return cancelQueuedCommand{requestID: requestID, response: response}
	})
}

// SetPermissionMode changes the Host's Claude permission mode through the same
// actor that serializes input and action decisions. The published state changes
// only after the Host confirms the SDK update.
func (c *Controller) SetPermissionMode(ctx context.Context, mode protocol.PermissionMode) error {
	if !mode.IsKnown() {
		return protocol.ErrInvalidPermissionMode
	}
	return c.sendControl(ctx, func(response chan error) actorCommand {
		return setPermissionModeCommand{mode: mode, response: response}
	})
}

// sendControl delivers a command that answers with an error only, so the three
// control commands share one liveness contract with Submit.
func (c *Controller) sendControl(ctx context.Context, build func(chan error) actorCommand) error {
	if !c.actorLive.Load() {
		return ErrSessionUnavailable
	}
	response := make(chan error, 1)
	command := build(response)
	select {
	case c.commands <- command:
	case <-ctx.Done():
		return ctx.Err()
	case <-c.actorDone:
		return ErrSessionUnavailable
	}

	select {
	case err := <-response:
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-c.actorDone:
		return ErrSessionUnavailable
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
		case request := <-c.worker.actions():
			// Pending actions share this select with Turn events on purpose: the
			// actor must never block on the worker while holding action state.
			c.handleActionRequest(&state, request)
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
	case resolveActionCommand:
		c.handleResolveAction(state, command)
	case cancelTurnCommand:
		c.handleCancelTurn(state, command)
	case cancelQueuedCommand:
		c.handleCancelQueued(state, command)
	case setPermissionModeCommand:
		c.handleSetPermissionMode(state, command)
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

func (c *Controller) handleSetPermissionMode(state *actorState, command setPermissionModeCommand) {
	if !command.mode.IsKnown() {
		command.response <- protocol.ErrInvalidPermissionMode
		return
	}
	host, ok := c.worker.host.(PermissionModeHost)
	if !ok {
		command.response <- errors.New("session host does not support runtime permission mode")
		return
	}
	if err := host.SetPermissionMode(context.Background(), command.mode); err != nil {
		command.response <- err
		return
	}
	state.snapshot.PermissionMode = command.mode
	c.publish(state, Event{
		Kind:           protocol.EventPermissionModeChanged,
		SessionID:      state.snapshot.SessionID,
		SessionStatus:  state.snapshot.SessionStatus,
		TurnStatus:     state.snapshot.Turn.Status,
		PermissionMode: command.mode,
	})
	command.response <- nil
}

// handleActionRequest admits at most one pending action per Session. Every
// rejection path must resolve the request before returning, otherwise the SDK
// read loop stays blocked forever (B2 §3.4).
func (c *Controller) handleActionRequest(state *actorState, request *projectclaude.ActionRequest) {
	spec := request.Spec()
	if state.stopping || state.current == nil || state.currentTerminal {
		request.Resolve(projectclaude.ActionDecision{Kind: projectclaude.DecisionDeny, Reason: denyReasonNoTurn})
		return
	}
	if state.pendingAction != nil {
		c.denyAndNotify(state, request, denyReasonConcurrent, UnsupportedConcurrentAction)
		return
	}
	if spec.Kind == projectclaude.ActionAskUserQuestion {
		// An empty question list is the Adapter's way of reporting a payload it
		// could not render; such an action could never be answered.
		if len(spec.Questions) == 0 {
			c.denyAndNotify(state, request, denyReasonUnrenderable, UnsupportedUnknownBlock)
			return
		}
		if !c.config.AskUserTextEnabled && hasFreeTextQuestion(spec.Questions) {
			c.denyAndNotify(state, request, denyReasonTextDisabled, UnsupportedTextDisabled)
			return
		}
	}

	state.nextActionSeq++
	actionID := fmt.Sprintf("%s-action-%d", state.current.RequestID, state.nextActionSeq)
	pending := newPendingAction(actionID, state.current.RequestID, spec)
	state.pendingAction = request
	state.pendingActionID = actionID
	state.snapshot.Turn.Status = TurnWaiting
	state.snapshot.Turn.PendingAction = pending
	c.publish(state, Event{
		Kind:          EventActionRequired,
		SessionID:     state.snapshot.SessionID,
		RequestID:     state.current.RequestID,
		Source:        state.current.Input.Source,
		TurnStatus:    TurnWaiting,
		PendingAction: pending,
	})
}

// denyAndNotify refuses an action the Session cannot present and tells clients
// why. The notice stays structurally empty: it describes a dropped action, so it
// must not look like one clients can address.
func (c *Controller) denyAndNotify(state *actorState, request *projectclaude.ActionRequest, reason string, code UnsupportedCode) {
	request.Resolve(projectclaude.ActionDecision{Kind: projectclaude.DecisionDeny, Reason: reason})
	c.publish(state, Event{
		Kind:      EventUnsupported,
		SessionID: state.snapshot.SessionID,
		RequestID: state.current.RequestID,
		Source:    state.current.Input.Source,
		Code:      code,
	})
}

// newPendingAction assigns the client-facing identifiers. The Adapter never sees
// them: actionId and questionId only exist so a client can name a decision
// target, and they are derived from actor state rather than the request ID
// generator so cancellation cannot consume request IDs.
func newPendingAction(actionID, requestID string, spec projectclaude.ActionSpec) *PendingAction {
	pending := &PendingAction{
		ActionID:              actionID,
		RequestID:             requestID,
		Kind:                  ActionKind(spec.Kind),
		ToolName:              spec.ToolName,
		Preview:               spec.Preview,
		AllowSessionAvailable: spec.AllowSessionAvailable,
	}
	if len(spec.Questions) == 0 {
		return pending
	}
	pending.Questions = make([]Question, 0, len(spec.Questions))
	for index, question := range spec.Questions {
		converted := Question{
			QuestionID:  fmt.Sprintf("%s-q%d", actionID, index+1),
			Header:      question.Header,
			Question:    question.Question,
			InputType:   QuestionInputType(question.InputType),
			MultiSelect: question.MultiSelect,
		}
		if len(question.Options) > 0 {
			converted.Options = make([]QuestionOption, 0, len(question.Options))
			for _, option := range question.Options {
				converted.Options = append(converted.Options, QuestionOption{Label: option.Label, Description: option.Description})
			}
		}
		pending.Questions = append(pending.Questions, converted)
	}
	return pending
}

func hasFreeTextQuestion(questions []projectclaude.Question) bool {
	for _, question := range questions {
		if question.InputType == projectclaude.QuestionText {
			return true
		}
	}
	return false
}

// handleResolveAction is the only path from a client decision to the blocked
// callback. A rejected resolution leaves the action pending so a second client
// can still answer it.
func (c *Controller) handleResolveAction(state *actorState, command resolveActionCommand) {
	pending := state.snapshot.Turn.PendingAction
	if state.pendingAction == nil || pending == nil || state.current == nil || state.pendingActionID != command.actionID {
		command.response <- ErrActionNotPending
		return
	}
	decision, err := buildActionDecision(pending, command.decision, command.answers)
	if err != nil {
		command.response <- err
		return
	}

	state.pendingAction.Resolve(decision)
	state.pendingAction = nil
	state.pendingActionID = ""
	state.snapshot.Turn.PendingAction = nil
	state.snapshot.Turn.Status = TurnRunning
	c.publish(state, Event{
		Kind:       EventActionResolved,
		SessionID:  state.snapshot.SessionID,
		RequestID:  pending.RequestID,
		Source:     state.current.Input.Source,
		TurnStatus: TurnRunning,
		Answers:    resolvedAnswers(pending, decision.Answers),
	})
	command.response <- nil
}

// resolvedAnswers pairs each question with the values just accepted, so
// action_resolved can echo the answer back to every client (D52). The values are
// the ones on their way to the SDK, never any part of a tool result; permission
// decisions carry no answers.
func resolvedAnswers(pending *PendingAction, ordered [][]string) []ResolvedAnswer {
	if pending.Kind != ActionAskUserQuestion || len(ordered) != len(pending.Questions) {
		return nil
	}
	answers := make([]ResolvedAnswer, len(pending.Questions))
	for index, question := range pending.Questions {
		answers[index] = ResolvedAnswer{
			QuestionID: question.QuestionID,
			Header:     question.Header,
			Question:   question.Question,
			Values:     append([]string(nil), ordered[index]...),
		}
	}
	return answers
}

// buildActionDecision enforces the shape contract between the action kind and
// the resolution. Deeper answer validation (option membership, single vs multi
// select arity) belongs to Transport, which owns the wire contract.
func buildActionDecision(pending *PendingAction, decision ActionDecision, answers []QuestionAnswer) (projectclaude.ActionDecision, error) {
	if pending.Kind == ActionAskUserQuestion {
		if decision != "" {
			return projectclaude.ActionDecision{}, ErrInvalidAction
		}
		ordered, err := orderAnswers(pending.Questions, answers)
		if err != nil {
			return projectclaude.ActionDecision{}, err
		}
		return projectclaude.ActionDecision{Kind: projectclaude.DecisionAnswer, Answers: ordered}, nil
	}

	if len(answers) > 0 {
		return projectclaude.ActionDecision{}, ErrInvalidAction
	}
	switch decision {
	case DecisionAllowOnce:
		return projectclaude.ActionDecision{Kind: projectclaude.DecisionAllowOnce}, nil
	case DecisionAllowSession:
		// "Allow for this Session" only exists when the SDK supplied permission
		// suggestions; the Adapter never invents its own permission rules.
		if !pending.AllowSessionAvailable {
			return projectclaude.ActionDecision{}, ErrInvalidAction
		}
		return projectclaude.ActionDecision{Kind: projectclaude.DecisionAllowSession}, nil
	case DecisionDeny:
		return projectclaude.ActionDecision{Kind: projectclaude.DecisionDeny, Reason: denyReasonUser}, nil
	default:
		return projectclaude.ActionDecision{}, ErrInvalidAction
	}
}

// orderAnswers turns the questionId-keyed client answers into the positional
// form the SDK expects. Answers may arrive in any order, but every question must
// be answered exactly once.
func orderAnswers(questions []Question, answers []QuestionAnswer) ([][]string, error) {
	if len(answers) != len(questions) {
		return nil, ErrInvalidAnswer
	}
	positions := make(map[string]int, len(questions))
	for index, question := range questions {
		positions[question.QuestionID] = index
	}
	ordered := make([][]string, len(questions))
	for _, answer := range answers {
		index, known := positions[answer.QuestionID]
		if !known || ordered[index] != nil || len(answer.Values) == 0 {
			return nil, ErrInvalidAnswer
		}
		ordered[index] = append([]string(nil), answer.Values...)
	}
	return ordered, nil
}

// handleCancelTurn ends the running Turn without ending the Session. The order
// matters: a blocked callback must be released through its decision channel,
// because the SDK control response for Interrupt travels on the same read loop
// the callback is holding (B2 §3.3).
func (c *Controller) handleCancelTurn(state *actorState, command cancelTurnCommand) {
	if state.stopping || state.current == nil || state.currentTerminal || state.current.RequestID != command.requestID {
		command.response <- ErrTurnNotCancellable
		return
	}

	state.currentCancelled = true
	interrupt := state.pendingAction == nil
	c.clearPendingAction(state, projectclaude.ActionDecision{
		Kind: projectclaude.DecisionDeny, Reason: denyReasonCancelled, Interrupt: true,
	})
	state.snapshot.Turn.Status = TurnCancelled
	state.snapshot.Turn.Error = nil
	state.currentTerminal = true
	state.preSessionEvents = nil
	c.publish(state, Event{
		Kind:       EventTurnCompleted,
		SessionID:  state.snapshot.SessionID,
		RequestID:  state.current.RequestID,
		Source:     state.current.Input.Source,
		TurnStatus: TurnCancelled,
	})
	if state.currentCancel != nil {
		state.currentCancel()
	}
	if interrupt {
		// Interrupt must not be awaited inside the actor: it is a round trip to
		// Claude, and the actor is the only consumer of the worker channels.
		go func() {
			_ = c.worker.interrupt(context.Background())
		}()
	}
	command.response <- nil
}

// handleCancelQueued drops a not-yet-started FIFO entry. It publishes no domain
// event: the queue is carried by the snapshot, and Transport acknowledges the
// command directly.
func (c *Controller) handleCancelQueued(state *actorState, command cancelQueuedCommand) {
	for index, queued := range state.queue {
		if queued.RequestID != command.requestID {
			continue
		}
		state.queue = append(state.queue[:index], state.queue[index+1:]...)
		c.syncQueue(state)
		command.response <- nil
		return
	}
	command.response <- ErrRequestNotQueued
}

// clearPendingAction is the single teardown path for a pending action. Every
// Turn or Session ending must go through it, so no callback is ever abandoned.
func (c *Controller) clearPendingAction(state *actorState, decision projectclaude.ActionDecision) {
	if state.pendingAction != nil {
		state.pendingAction.Resolve(decision)
	}
	state.pendingAction = nil
	state.pendingActionID = ""
	state.snapshot.Turn.PendingAction = nil
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
	c.syncQueue(state)

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
	c.syncQueue(state)
	state.current = &next
	turnContext, cancel := context.WithCancel(context.Background())
	state.currentCancel = cancel
	state.currentTerminal = false
	state.currentCancelled = false
	state.turnStartedPublished = false
	state.snapshot.Turn = TurnSnapshot{
		Status:        TurnRunning,
		RequestID:     next.RequestID,
		Source:        next.Input.Source,
		PendingAction: nil,
	}

	// A5.2 §6：sessionId 已预留但未确认时暂缓 turn_started，init 确认后
	// 由 handleWorkerEvent 按既有顺序补发。
	if state.sessionConfirmed {
		state.turnStartedPublished = true
		c.publish(state, Event{
			Kind:       EventTurnStarted,
			SessionID:  state.snapshot.SessionID,
			RequestID:  next.RequestID,
			Source:     next.Input.Source,
			Text:       next.Input.Text,
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
		if state.sessionConfirmed {
			return
		}
		if state.snapshot.SessionID != "" && state.snapshot.SessionID != event.Adapter.SessionID {
			// A5.2 §4：Claude init 返回的 ID 必须与预留 ID 一致，不一致按
			// Session 级错误处理，禁止迁移。
			c.failSession(state, errors.New(fmt.Sprintf("session ID 与预留不一致: 预留 %q, Claude 返回 %q",
				state.snapshot.SessionID, event.Adapter.SessionID)))
			return
		}
		state.snapshot.SessionID = event.Adapter.SessionID
		state.sessionConfirmed = true
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
				Text:       state.current.Input.Text,
				TurnStatus: TurnRunning,
			})
		}
		buffered := state.preSessionEvents
		state.preSessionEvents = nil
		for _, bufferedEvent := range buffered {
			c.publishAdapterOutput(state, bufferedEvent)
		}
	case projectclaude.EventText, projectclaude.EventContentBlock, projectclaude.EventUnsupported:
		if !state.sessionConfirmed {
			state.preSessionEvents = append(state.preSessionEvents, event)
			return
		}
		c.publishAdapterOutput(state, event)
	case projectclaude.EventTurnCompleted:
		state.snapshot.Turn.Status = TurnCompleted
		state.snapshot.Turn.Error = nil
		c.clearPendingAction(state, projectclaude.ActionDecision{
			Kind: projectclaude.DecisionDeny, Reason: denyReasonTerminated,
		})
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

	// A Turn the Controller itself cancelled reports context.Canceled, which is
	// Session-scoped by default; treating it as a Session failure would end the
	// Session that cancel_turn is explicitly meant to keep alive.
	if !state.stopping && !state.currentCancelled {
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
	state.currentCancelled = false
	state.turnStartedPublished = false
	state.preSessionEvents = nil
	c.clearPendingAction(state, projectclaude.ActionDecision{
		Kind: projectclaude.DecisionDeny, Reason: denyReasonTerminated,
	})
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

// publishAdapterOutput forwards one streamed Adapter output. Text, structured
// blocks and degradation notices share the correlation guard and the
// pre-session buffering rule, so they also share this entry point.
func (c *Controller) publishAdapterOutput(state *actorState, event workerEvent) {
	switch event.Adapter.Kind {
	case projectclaude.EventText:
		c.appendText(state, event)
	case projectclaude.EventContentBlock:
		c.publishContentBlock(state, event)
	case projectclaude.EventUnsupported:
		if state.current == nil || state.current.RequestID != event.RequestID || state.currentTerminal {
			return
		}
		c.publish(state, Event{
			Kind:      EventUnsupported,
			SessionID: state.snapshot.SessionID,
			RequestID: state.current.RequestID,
			Source:    state.current.Input.Source,
			Code:      UnsupportedCode(event.Adapter.Code),
		})
	}
}

// publishContentBlock assigns the client-facing block ID. Like actionId the ID is
// derived from actor state instead of coming from the Adapter, because the
// Controller is the only holder of seq and therefore of the order clients
// correlate against (B2 §4.2). A structured block never enters the snapshot: the
// design keeps it event-only, so a reconnecting client is not replayed history.
func (c *Controller) publishContentBlock(state *actorState, event workerEvent) {
	if state.current == nil || state.current.RequestID != event.RequestID || state.currentTerminal {
		return
	}
	if event.Adapter.Block == nil {
		// An Adapter that reports a block without one has nothing to present; the
		// safety notice is the only honest thing left to send.
		c.publish(state, Event{
			Kind:      EventUnsupported,
			SessionID: state.snapshot.SessionID,
			RequestID: state.current.RequestID,
			Source:    state.current.Input.Source,
			Code:      UnsupportedUnknownBlock,
		})
		return
	}
	state.nextBlockSeq++
	blockID := fmt.Sprintf("%s-block-%d", state.current.RequestID, state.nextBlockSeq)
	c.publish(state, Event{
		Kind:      EventContentBlock,
		SessionID: state.snapshot.SessionID,
		RequestID: state.current.RequestID,
		Source:    state.current.Input.Source,
		Block:     newContentBlock(blockID, *event.Adapter.Block),
	})
}

// newContentBlock copies the Adapter view into the client view field by field
// rather than by struct conversion, so a future Adapter field cannot reach
// clients without passing the B2 §4.2 whitelist review first.
func newContentBlock(blockID string, block projectclaude.ContentBlock) *ContentBlock {
	converted := &ContentBlock{
		ID:        blockID,
		Kind:      BlockKind(block.Kind),
		Thinking:  block.Thinking,
		ToolUseID: block.ToolUseID,
		ToolClass: ToolClass(block.ToolClass),
		MCPServer: block.MCPServer,
		ToolName:  block.ToolName,
		Preview:   block.Preview,
	}
	if block.IsError != nil {
		// Copy the value instead of the pointer: the published event outlives the
		// Adapter event and must not share mutable state across layers.
		isError := *block.IsError
		converted.IsError = &isError
	}
	return converted
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
	c.clearPendingAction(state, projectclaude.ActionDecision{
		Kind: projectclaude.DecisionDeny, Reason: denyReasonTerminated,
	})
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
	// Defensive: failSession can be reached without a running Turn, so the
	// teardown inside failTurn is not guaranteed to have run.
	c.clearPendingAction(state, projectclaude.ActionDecision{
		Kind: projectclaude.DecisionDeny, Reason: denyReasonTerminated,
	})
	state.snapshot.SessionStatus = SessionFailed
	state.snapshot.SessionError = &ErrorInfo{Code: "session_failed", Message: err.Error()}
	state.queue = nil
	c.syncQueue(state)
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
	switch event.Kind {
	case EventTurnStarted:
		if state.snapshot.SessionTitle == "" {
			state.snapshot.SessionTitle = protocol.SessionTitleFromInput(event.Text)
			event.SessionTitle = state.snapshot.SessionTitle
		}
		fallthrough
	case EventTurnCompleted, EventTurnFailed, EventActionRequired, EventSessionFailed, EventSessionStopped:
		state.snapshot.LastActivityAt = time.Now().UTC().Format(time.RFC3339Nano)
		event.LastActivityAt = state.snapshot.LastActivityAt
	}
	state.snapshot.Seq++
	event.Seq = state.snapshot.Seq
	for subscriberID, subscriber := range state.subscribers {
		published := event
		published.Error = cloneErrorInfo(event.Error)
		published.PendingAction = clonePendingAction(event.PendingAction)
		published.Answers = cloneResolvedAnswers(event.Answers)
		if event.Block != nil {
			block := *event.Block
			published.Block = &block
		}
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

// syncQueue republishes the whole FIFO view after any queue mutation. Routing it
// through setQueuedRequests keeps queueDepth and queuedRequests written together,
// so the two views can never disagree (B2 §4.4).
func (c *Controller) syncQueue(state *actorState) {
	if len(state.queue) == 0 {
		setQueuedRequests(&state.snapshot, nil)
		return
	}
	items := make([]QueuedRequest, 0, len(state.queue))
	for _, queued := range state.queue {
		items = append(items, newQueuedRequest(c.config, queued.RequestID, queued.Input))
	}
	setQueuedRequests(&state.snapshot, items)
}

func cloneSnapshot(snapshot Snapshot) Snapshot {
	cloned := snapshot
	cloned.SessionError = cloneErrorInfo(snapshot.SessionError)
	cloned.Turn.Error = cloneErrorInfo(snapshot.Turn.Error)
	cloned.Turn.PendingAction = clonePendingAction(snapshot.Turn.PendingAction)
	if snapshot.QueuedRequests != nil {
		cloned.QueuedRequests = append([]QueuedRequest(nil), snapshot.QueuedRequests...)
	}
	return cloned
}

// clonePendingAction deep-copies the questions and options as well, so a
// subscriber can never mutate the state the actor is still holding.
func clonePendingAction(action *PendingAction) *PendingAction {
	if action == nil {
		return nil
	}
	cloned := *action
	if action.Questions == nil {
		return &cloned
	}
	cloned.Questions = make([]Question, len(action.Questions))
	for index, question := range action.Questions {
		cloned.Questions[index] = question
		if question.Options != nil {
			cloned.Questions[index].Options = append([]QuestionOption(nil), question.Options...)
		}
	}
	return &cloned
}

// cloneResolvedAnswers keeps each subscriber's copy independent, matching how the
// other event payloads are cloned before delivery.
func cloneResolvedAnswers(answers []ResolvedAnswer) []ResolvedAnswer {
	if answers == nil {
		return nil
	}
	cloned := make([]ResolvedAnswer, len(answers))
	for index, answer := range answers {
		cloned[index] = answer
		cloned[index].Values = append([]string(nil), answer.Values...)
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
	c.syncQueue(state)
	state.preSessionEvents = nil
	// Stopping releases the callback with Interrupt set: the Turn is going away,
	// and the SDK read loop must unwind rather than wait for a new decision.
	c.clearPendingAction(state, projectclaude.ActionDecision{
		Kind: projectclaude.DecisionDeny, Reason: denyReasonTerminated, Interrupt: true,
	})
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
