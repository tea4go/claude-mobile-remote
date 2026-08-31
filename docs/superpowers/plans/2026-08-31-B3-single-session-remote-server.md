# B3 Single Session Remote Server Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox syntax for tracking.

**Goal:** Move shared Go protocol types into public protocol/ and expose the existing single-session pairing/channel/transport composition through importable remote.Server without changing the wire contract or stopping the host Session. Standalone CLI also accepts an explicit Claude working directory for new and resumed sessions.

**Architecture:** protocol/ is the single Go source of truth for JSON-bearing Session, Turn, event, input, action, and service-config types. internal/session keeps source compatibility through aliases; internal/channel and internal/transport consume protocol. remote/ owns the public Controller/Server API and composes existing internals; the CLI remains responsible for starting and stopping its Controller.

**Tech Stack:** Go 1.25.3, Gorilla WebSocket, existing claude-agent-sdk-go, Go standard library tests.

---

## File Map

- Create protocol/model.go, protocol/event.go, protocol/input.go, protocol/pairing.go, protocol/preview.go: the single public source for JSON-bearing state, event, input, action, pairing, service-config, and preview types.
- Modify internal/session/model.go, event.go, input.go, controller.go: aliases to protocol; retain actor-only validation and queue helpers. Publicly meaningful host error sentinels also alias protocol errors.
- Modify internal/claude/preview.go: delegate shared truncation/default to protocol without duplicating logic.
- Modify internal/channel/channel.go and internal/transport/protocol.go, connection.go, server.go: consume protocol types.
- Create remote/remote.go, remote/remote_test.go, remote/external_contract_test.go: public API, lifecycle, rollback, shutdown, and external importability tests.
- Modify cmd/claude-mobile-remote/main.go, run.go, run_test.go; delete cmd/claude-mobile-remote/remote.go: migrate Standalone CLI.
- Modify cmd/claude-mobile-remote/config.go, config_test.go, main.go, and internal/claude/host.go, host_test.go: add validated `--cwd` support and pass it to both new and resume SDK clients.
- Create docs/testing/B3-result.md; modify HANDOFF.md: record gate evidence and handoff.

## Public Contract

protocol/ owns the exact existing JSON fields and values. These remain aliases in internal/session/pairing, not copies: Snapshot, Event, Input, ContentBlock, PendingAction, Question, QuestionAnswer, ActionDecision, ServiceConfig, and Pairing. It also owns the public error sentinels needed by an external Controller to preserve stable Transport command-result mapping.

remote/ exposes Controller methods using protocol.Input, protocol.Snapshot, protocol.Event, protocol.ActionDecision, and protocol.QuestionAnswer. Config contains SessionID, Controller, ListenAddress, AdvertiseURL, PairTokenTTL, PairingPersistenceFile, AllowedOrigins, AuthTimeout, CloseTimeout, ReadLimit, and protocol.ServiceConfig. Server exposes New(Config), Start() (protocol.Pairing, error), Pairings() <-chan protocol.Pairing, BeginShutdown(), and Close(context.Context) error.

protocol.Pairing is the public version of the existing pairing payload shape. Start performs only remote startup and initial pairing; it never calls host Controller Start/Stop. Close unregisters the one Channel and closes remote resources; it never stops the host Session.

### Task 1: Add failing protocol ownership tests

**Files:** Create protocol/model_test.go, protocol/event_test.go, protocol/input_test.go, protocol/pairing_test.go, and protocol/external_json_test.go.

- [ ] Step 1: Marshal representative protocol.Snapshot, protocol.Event containing a ContentBlock and PendingAction, and protocol.Input. Assert existing field names (sessionId, pendingAction, toolUseId, isError, questionId), omission behavior, and compile-time public Controller-shaped signatures.
- [ ] Step 2: Run:
  go test ./protocol -run 'Test(ProtocolJSON|Public)' -count=1
  Expected: FAIL because protocol and its public types do not exist.

### Task 2: Move protocol types to the public package

**Files:** Create protocol/model.go, event.go, input.go, pairing.go, preview.go. Modify internal/session/model.go, event.go, input.go, controller.go, internal/pairing/payload.go, and internal/claude/preview.go.

- [ ] Step 1: Move definitions currently in Session model/event/input into protocol. Move ActionDecision and QuestionAnswer from Session Controller, and Payload from pairing.Payload, into protocol. Move the public command-mapping error sentinels currently in Session into protocol. Move DefaultToolPreviewMaxRunes, TruncatePreview, and ellipsis behavior into protocol/preview.go.
- [ ] Step 2: Replace moved internal definitions with type and constant aliases such as type Snapshot = protocol.Snapshot and const SessionReady = protocol.SessionReady; make internal/pairing.Payload an alias to protocol.Pairing. Alias the stable Session errors to protocol errors so external Controllers can return them. Keep normalization, actor-only helpers, and queue construction local. Replace Snapshot.setQueuedRequests with an internal helper assigning QueuedRequests and QueueDepth together, because Go cannot define a method on an alias to a non-local type. Keep internal/claude.TruncatePreview as a wrapper calling protocol.TruncatePreview.
- [ ] Step 3: Run:
  gofmt -w protocol internal/session internal/claude/preview.go
  go test ./protocol ./internal/session ./internal/claude -count=1
  Expected: PASS, including new JSON tests and existing Session/preview tests.

### Task 3: Repoint Channel and Transport at the single protocol package

**Files:** Modify internal/channel/channel.go, internal/transport/protocol.go, connection.go, server.go, and affected tests only for moved symbol imports.

- [ ] Step 1: Add a fixture whose methods use protocol.Input, protocol.Snapshot, protocol.Event, protocol.ActionDecision, and protocol.QuestionAnswer; assert it satisfies the Channel Controller interface.
- [ ] Step 2: Change Channel and Transport signatures/payloads to use protocol directly. Move action/question wire types out of Transport; retain internal aliases only if existing command-client code needs old names. Map public protocol errors in commandError. Keep envelope validation, Version 2, and JSON output unchanged.
- [ ] Step 3: Run:
  gofmt -w internal/channel internal/transport cmd/channel-test-client
  go test ./internal/channel ./internal/transport ./cmd/channel-test-client -count=1
  Expected: PASS with no protocol JSON differences.

### Task 4: Add the public remote.Server composition

**Files:** Create remote/remote.go, remote/remote_test.go, and remote/external_contract_test.go.

- [ ] Step 1: Construct remote.New with SessionID session-1, a public Controller fixture, ListenAddress 127.0.0.1:0, AdvertiseURL ws://127.0.0.1:8765/ws, and PairTokenTTL time.Hour. Assert Start returns the configured Session ID, does not call Controller Start/Stop, and Close calls neither. Add rollback tests for transport-start and channel-registration failures. Put the external fixture in package remote_test and import only remote and protocol.
- [ ] Step 2: Run:
  go test ./remote -run 'Test(Server|External)' -count=1
  Expected: FAIL because the package/API is missing.
- [ ] Step 3: Implement remote.Server with Pairing Manager, Registry, Transport Server, current Channel, Controller, and rotation lifecycle. New validates Session ID/Controller and constructs defaults. Start registers exactly one Channel, starts Transport, issues Pairing, and rolls back registration/resources on failure. Pairings returns the bounded rotation channel. BeginShutdown and Close mirror current runtime shutdown, additionally unregistering the Session Channel and never calling Controller.Stop. Use an internal transport seam for deterministic rollback/timeout tests; production uses transport.NewServer.
- [ ] Step 4: Run:
  gofmt -w remote
  go test ./remote -count=1
  Expected: PASS, including external-contract and lifecycle tests.

### Task 5: Migrate Standalone CLI to the public component

**Files:** Modify cmd/claude-mobile-remote/main.go, run.go, run_test.go; delete cmd/claude-mobile-remote/remote.go.

- [ ] Step 1: Extend runtime-builder coverage to assert the returned service uses public Server behavior and initial pairing is returned by Start, not a command-local Activate call.
- [ ] Step 2: Run:
  go test ./cmd/claude-mobile-remote -run 'Test(Main|Run|Runtime)' -count=1
  Expected: FAIL because the builder still returns the command-local runtime and the service interface expects Activate.
- [ ] Step 3: Build remote.Config from CLI config, pass the existing Session Controller directly through protocol aliases, and make runWithRemote consume Start() (protocol.Pairing, error). Print the initial Pairing where the old Activate result was printed. Preserve deferred order: CLI stops Controller first, then closes remote service; public Server never stops Controller.
- [ ] Step 4: Delete the duplicate runtime after all callers use remote.Server. Run:
  gofmt -w cmd/claude-mobile-remote
  go test ./cmd/claude-mobile-remote -count=1
  Expected: PASS with existing fake-host and shutdown tests intact.

### Task 6: Full regression and B3 gate evidence

**Files:** Create docs/testing/B3-result.md; modify HANDOFF.md.

- [ ] Step 1: Run:
  gofmt -l .
  GOFLAGS=-buildvcs=false go vet ./...
  GOFLAGS=-buildvcs=false go test -count=1 ./...
  git diff --check
  Expected: no output from gofmt/vet/diff-check and all packages pass. git diff --check is the only Git command used and is permitted by project rules.
- [ ] Step 2: Verify source ownership with:
  rg -n '^type (Snapshot|Event|Input|ContentBlock|PendingAction|Question|QuestionAnswer|ActionDecision|ServiceConfig)' protocol internal/session internal/transport
  Expected: moved definitions appear in protocol; internal/session and internal/pairing contain aliases only and no second struct definition exists in Session, Pairing, or Transport.
- [ ] Step 3: Record exact commands, package counts, JSON compatibility, external-contract, lifecycle/shutdown results, and unverified real-Claude/WhaleTerm/C2 items. Never include Pair Tokens, Claude prompts, tool inputs, or session contents.
- [ ] Step 4: Replace the B3 current-execution entry in HANDOFF.md with actual completed work, verification output, remaining gaps, and the next W1/C2 relationship. Do not claim WhaleTerm integration itself is complete.

### Task 7: Add Standalone CLI custom working directory

**Files:** Modify cmd/claude-mobile-remote/config.go, config_test.go, main.go, and internal/claude/host.go, host_test.go; update README.md and docs/testing/B3-result.md.

- [x] Step 1: Add failing tests for parsing `--cwd`, rejecting an empty/non-directory path, and preserving the selected path in new and resume Host construction.
- [x] Step 2: Run the focused tests and confirm they fail because the CLI flag and Host option do not exist.
- [x] Step 3: Implement `--cwd`: resolve and validate the path during CLI configuration, add `claude.WithCwd` to SDK options for both new and resume modes, and keep the default behavior as the process working directory.
- [x] Step 4: Run focused tests, full Go verification, and update the B3 evidence with the new command and result. Do not change external Session takeover behavior.
