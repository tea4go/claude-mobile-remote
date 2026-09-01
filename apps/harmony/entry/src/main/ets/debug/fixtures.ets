export const FIXTURE_NOW_MS = Date.parse('2026-08-31T00:00:00Z')

export const VALID_PAIRING_PAYLOAD = `{"version":1,"wsUrl":"wss://example.test/ws","sessionId":"session-1","pairToken":"pair-token-1","expiresAt":"2026-09-01T00:00:00Z"}`

export const EXPIRED_PAIRING_PAYLOAD = `{"version":1,"wsUrl":"ws://127.0.0.1:8765/ws","sessionId":"session-1","pairToken":"pair-token-2","expiresAt":"2020-01-01T00:00:00Z"}`

export const PRETURN_SNAPSHOT_MESSAGE = `{"version":2,"type":"snapshot","sessionId":"session-1","commandId":"","requestId":"","seq":0,"payload":{"connectionStatus":"subscribed","snapshot":{"sessionId":"session-1","sessionStatus":"starting","sessionError":null,"seq":0,"turn":{"status":"idle","requestId":"","source":"","message":"","error":null,"pendingAction":null},"queueDepth":0,"queuedRequests":null}}}`

export const QUEUED_SNAPSHOT_MESSAGE = `{"version":2,"type":"snapshot","sessionId":"session-1","commandId":"","requestId":"req-1","seq":7,"payload":{"connectionStatus":"subscribed","snapshot":{"sessionId":"session-1","sessionStatus":"ready","sessionError":null,"seq":7,"turn":{"status":"waiting","requestId":"req-1","source":"mobile","message":"build the project","error":null,"pendingAction":{"actionId":"act-1","requestId":"req-1","kind":"tool_approval","toolName":"Bash","preview":"devecocli build --modules entry@default","allowSessionAvailable":true}},"queueDepth":2,"queuedRequests":[{"requestId":"req-2","source":"mobile","preview":"show logs"},{"requestId":"req-3","source":"terminal","preview":"list pending tasks"}]}}}`

export const CONTENT_BLOCK_EVENT_MESSAGE = `{"version":2,"type":"event","sessionId":"session-1","commandId":"","requestId":"req-1","seq":8,"payload":{"event":{"seq":8,"kind":"content_block","sessionId":"session-1","requestId":"req-1","source":"mobile","text":"","sessionStatus":"","turnStatus":"","error":null,"block":{"id":"block-1","kind":"tool_use","toolUseId":"tool-1","toolClass":"mcp","toolName":"search","preview":"search docs","mcpServer":"docs"}}}}`

export const ACTION_REQUIRED_EVENT_MESSAGE = `{"version":2,"type":"event","sessionId":"session-1","commandId":"","requestId":"req-4","seq":9,"payload":{"event":{"seq":9,"kind":"action_required","sessionId":"session-1","requestId":"req-4","source":"terminal","text":"","sessionStatus":"","turnStatus":"waiting","error":null,"pendingAction":{"actionId":"act-2","requestId":"req-4","kind":"ask_user_question","toolName":"AskUserQuestion","allowSessionAvailable":false,"questions":[{"questionId":"q-1","header":"Scope","question":"Choose a target","inputType":"choice","multiSelect":false,"options":[{"label":"entry","description":"Build only entry"}]}]}}}}`

export const INVALID_TURN_STARTED_EVENT_MESSAGE = `{"version":2,"type":"event","sessionId":"session-1","commandId":"","requestId":"","seq":11,"payload":{"event":{"seq":11,"kind":"turn_started","sessionId":"session-1","requestId":"","source":"","text":"","sessionStatus":"","turnStatus":"completed","error":null}}}`

export const MISSING_ERROR_EVENT_MESSAGE = `{"version":2,"type":"event","sessionId":"session-1","commandId":"","requestId":"req-5","seq":12,"payload":{"event":{"seq":12,"kind":"text_delta","sessionId":"session-1","requestId":"req-5","source":"mobile","text":"delta","sessionStatus":"","turnStatus":""}}}`

export const UNSUPPORTED_MESSAGE = `{"version":2,"type":"unsupported","sessionId":"session-1","commandId":"","requestId":"","seq":10,"payload":{"code":"unknown_block"}}`
