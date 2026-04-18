# V2 Development Rules

## 1. Purpose

This document defines how V2 is built.

It is the rulebook for:

- prompt shaping
- epic and story breakdown
- repository boundaries
- testing expectations
- reuse rules for V1 code
- server-first development workflow

This document is intended to stay short, stable, and operational.

## 2. Core Principles

- Build V2 through small, bounded stories rather than global prompts.
- Keep server and client work separated unless a later integration story explicitly combines them.
- Prefer narrow feature entry points and low dependency fan-out.
- Use deterministic automated tests as the main proof of correctness.
- Put meaningful comments in the code where they help humans and LLMs understand intent and boundaries.
- Avoid documentation churn; code comments and tests are preferred over large drifting implementation writeups.

## 3. Runtime Topology

V2 starts with three server processes:

1. `lobby`
2. `matchmaker`
3. `game-session`

Initial transport choices:

- `lobby`: HTTP + JSON
- `matchmaker`: HTTP + JSON
- `game-session`: WebSocket + JSON
- `lobby` assignment lookup: HTTP polling for the first milestone
- `game-session` internal bootstrap: HTTP + JSON for session provisioning

Rationale:

- `lobby` and `matchmaker` are thin control services.
- `game-session` is event-driven and needs push-based multiplayer messaging.
- `game-session` may also expose a thin internal bootstrap endpoint for service-to-service session provisioning.

Bootstrap identity rules:

- `client_id` is the stable bootstrap identity issued by `lobby`.
- `seat` is the authoritative in-session identity issued by `game-session`.
- `session_player_token` is a session-scoped reconnect token issued by `game-session`.
- `character_id` is gameplay setup data, not the answer to "who am I in this session?"

Reconnect is a separate future epic.

The first milestone must not implement reconnect behavior, but it must preserve the insertion points needed for a later reconnect story.

## 4. Repository Boundary Rules

V2 uses a hybrid structure:

- feature-oriented folders for application work
- shared domain folders for core concepts
- separate transport folders for protocol and runtime adapters

Implications:

- feature prompts can target a narrow area like `drawcard`
- shared concepts like `deck`, `player`, or `session` are not duplicated across features
- transport code does not own game rules

## 5. Epic and Story Rules

### Epic

An epic is a meaningful capability or milestone.

Examples:

- `connect_two_players`
- `draw_card`
- `roll_offensive_dice`

### Story

A story is one bounded part of an epic and includes the code and the focused tests for that part.

A story should:

- stay on one side of the stack when possible
- focus on one bounded piece of work
- include the deterministic test that proves that slice

Examples inside the `draw_card` epic:

- server-side draw-card domain behavior plus focused tests
- server-side draw-card scenario flow plus scenario test
- draw-card JSON contract plus contract tests
- client-side card-hand state update plus client test
- client-side drawn-card rendering plus UI test

## 6. Story Composition Rules

A story normally changes:

- one bounded implementation slice
- its focused tests

The final story in an epic may add or update an end-to-end test for that epic.

Default sequencing for a gameplay capability:

1. server/domain behavior
2. server/application wiring
3. server scenario proof
4. protocol contract
5. client state handling
6. client rendering
7. end-to-end verification

Protocol work should be formalized after server behavior is proven internally and before client work begins.

## 7. Testing Rules

Testing is priority 1. A story is not complete until the relevant tests for that story are added or updated and the full automated suite passes.

### Server

The truth for server correctness is:

- Go module tests for focused behavior
- Go scenario/integration tests for feature flows

Shell scripts may exist for convenience, but they are not the main correctness layer.

### Client

The truth for client correctness is:

- deterministic Godot-side tests
- fixture-driven tests against stable protocol messages where possible

### Test Granularity

Do not write trivial tests for passive getters, setters, or low-value wiring.

Write tests for:

- rule behavior
- state transitions
- event emission
- protocol shape
- user-visible feature outcomes

### Godot/Go Battle Authority Test Layers

For the Godot/Go battle authority path, every story should be covered at the right combination of these layers:

1. Focused Go rule/module tests for meaningful behavior below the JSON boundary, such as rolling dice, applying damage, resolving an ability, or advancing a segment.
2. Go authority command tests for command JSON in and result JSON out, without Godot or C++.
3. Godot headless integration tests for the full `Godot -> C++ -> Go -> C++ -> Godot` loop, without UI button clicks.

Manual Godot UI checks are allowed for presentation work and spikes, but they are not a replacement for automated tests.

The C++ GDExtension bridge must remain a thin JSON transport layer. Gameplay stories should not modify it. If C++ bridge changes appear necessary, stop and explain the reason for explicit review before implementing them.

The detailed policy for Godot/Go battle-authority stories is in:

```text
docs/v2-planning/godot_pve/03_story_testing_policy.md
```

## 8. Determinism Rules

Where needed, code should support injected deterministic dependencies such as:

- fixed deck order
- fake RNG
- fake clock
- fake ID generator
- fake message sink
- fake websocket or client harness

If a feature cannot be tested deterministically, the design is not ready yet.

## 9. Server-First Workflow

For gameplay work:

- finish the server slice first
- lock the protocol next
- then start client work

Do not start client implementation for a feature while server behavior and message shape are still fluid.

## 10. V1 Reuse Rules

V1 reuse is allowed opportunistically.

A V1 component may be reused only if:

- the code is acceptable in quality
- it fits or can be moved into the V2 structure
- it follows V2 boundary rules after adaptation
- it is covered by V2 tests

If reused code does not fit those rules cleanly, rewrite it.

## 11. Definition Of Done

A story is done when:

- the bounded code slice is complete
- automated tests for that story pass
- the full relevant automated suite has been run after the final change

An epic is done when:

- all intended stories for the epic are complete
- the final integration or end-to-end proof for the epic passes

## 12. Prompt Shaping Template

Broad prompts should be refined into this shape before coding:

- Epic
- Goal
- Current scope
- Side of stack: `server` or `client`
- Story candidate list
- Acceptance criteria per story
- Required tests per story
- Touched paths
- Open questions

## 13. Prompt Refinement Rules

A prompt-refinement tool or skill should turn a broad request into:

- `Epic`
- numbered `Stories`
- `Acceptance Criteria`
- `Tests`
- `Touched Paths`
- `Open Questions`

The refinement should prefer:

- server-first sequencing
- one bounded story at a time
- deterministic tests inside each story
- no automatic cross-stack coupling unless explicitly requested

## 14. First Milestone Rule

The first V2 milestone is server-only:

- two clients connect through `lobby`
- `matchmaker` pairs them
- each client polls `lobby` for assignment and receives session connection details
- both connect to `game-session`
- both are seated successfully
- each client receives `session_joined` with `client_id`, `seat`, and `session_player_token`
- both receive a `session-start` event

Client work for this flow happens later as a separate epic.

The first milestone should keep `session-start` minimal:

- `session_id`
- `match_id`
- player seat map
- `your_client_id`
- `your_seat`
- `session_player_token`

Do not use `character_id` as the client's identity hook.

The first milestone must remain reconnect-ready:

- do not couple session identity to websocket connection identity alone
- do not hide seat assignment inside transport-only state
- do not make `session_start` the only place where identity is available
- keep `session_player_token` issued by `game-session` even before reconnect is implemented
