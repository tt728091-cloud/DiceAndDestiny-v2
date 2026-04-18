# Story 2: `matchmaker/queuematchrequest`

## Story Goal

Create the `matchmaker` queue intake endpoint so `lobby` can submit a match request and receive a queued response when no pair exists yet.

This story owns request intake and queue placement only. It does not perform pairing yet.

## Scope

- add `POST /v1/matchmaker/requests`
- validate queue requests
- store unmatched requests as queued
- return the queued response
- add focused automated tests

## Exact Contract

### Request

`POST /v1/matchmaker/requests`

```json
{
  "client_id": "client-1",
  "character_id": "test-character"
}
```

### Queued Response

`202 Accepted`

```json
{
  "status": "queued",
  "client_id": "client-1"
}
```

## Acceptance Criteria

- valid queue requests are accepted from `lobby`
- unmatched requests are stored as queued
- queued response shape matches the contract
- invalid requests are rejected cleanly
- focused automated tests pass

## Tests Required

- Go test for queue request validation
- Go test for queued response shape
- Go test proving an unmatched request is stored as queued

## Touched Paths

```text
v2/server/matchmaker/internal/app/queuematchrequest/
v2/server/matchmaker/internal/transport/http/
```

## Rules And Constraints

- server only
- do not implement pair formation in this story
- keep queue storage behind a clean boundary so tests can control it
- keep request validation deterministic and explicit

## Out Of Scope

- `pairplayers`
- `assignsession`
- calling back to `lobby`

## Assumptions For Now

- `client_id` and `character_id` are both required
- queue can be in-memory for the first milestone if the boundary is clean

## Completion Proof

This story is complete when `matchmaker` can accept one request, store it as queued, and return the expected queued response with passing focused tests.
