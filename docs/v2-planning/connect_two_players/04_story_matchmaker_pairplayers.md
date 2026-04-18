# Story 4: `matchmaker/pairplayers`

## Story Goal

Create the deterministic pairing logic that turns two queued requests into one formed match with a `match_id`.

This story owns pairing only. It does not assign a session or notify `lobby`.

## Scope

- take two compatible queued requests
- pair them deterministically
- create a `match_id`
- add focused automated tests

## Acceptance Criteria

- two compatible queued requests are paired deterministically
- pairing creates a `match_id`
- pairing removes or transitions queued state appropriately
- focused automated tests pass

## Tests Required

- Go test for pairing two queued requests
- Go test for deterministic first-match behavior
- Go test for queue state after pairing

## Touched Paths

```text
v2/server/matchmaker/internal/app/pairplayers/
```

## Rules And Constraints

- server only
- do not assign a `session_id` here
- keep pairing independent from transport and callback concerns
- keep the pair result explicit so later stories can consume it cleanly

## Out Of Scope

- session assignment
- lobby notification
- queue intake endpoint

## Assumptions For Now

- first milestone can use simple first-in-first-out compatibility
- advanced matchmaking rules are a later epic

## Completion Proof

This story is complete when two queued requests can be paired deterministically into a formed match with passing focused tests.
