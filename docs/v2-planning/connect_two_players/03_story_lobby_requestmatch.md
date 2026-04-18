# Story 3: `lobby/requestmatch`

## Story Goal

Create the `lobby` match-request endpoint so a registered client can request a match and `lobby` can forward the correct payload to `matchmaker`.

This story owns request handling and forwarding only. It does not pair clients or expose assignment lookup.

## Scope

- add `POST /v1/lobby/clients/{client_id}/match-requests`
- validate the incoming request
- call `matchmaker` with the correct payload
- return the `searching` response
- add focused automated tests

## Exact Contract

### Request

`POST /v1/lobby/clients/{client_id}/match-requests`

```json
{
  "character_id": "test-character"
}
```

### Success Response

`202 Accepted`

```json
{
  "client_id": "client-1",
  "status": "searching"
}
```

### Forwarded Payload To Matchmaker

`POST /v1/matchmaker/requests`

```json
{
  "client_id": "client-1",
  "character_id": "test-character"
}
```

## Acceptance Criteria

- valid client match requests are accepted
- `lobby` forwards the correct payload to `matchmaker`
- client receives `status=searching`
- invalid requests are rejected cleanly
- focused automated tests pass

## Tests Required

- Go test for request validation
- Go test with a fake `matchmaker` client verifying forwarded payload
- Go test for returned `searching` response shape

## Touched Paths

```text
v2/server/lobby/internal/app/requestmatch/
v2/server/lobby/internal/transport/http/
```

## Rules And Constraints

- server only
- do not implement pairing here
- use a fake `matchmaker` client in focused tests rather than a live service
- keep the forwarding boundary explicit so later stories can swap implementations cleanly

## Out Of Scope

- pairing logic
- assignment callback handling
- assignment polling

## Assumptions For Now

- the `client_id` already exists from Story 1
- `character_id` is required for the first milestone

## Completion Proof

This story is complete when `lobby` can accept a valid request, forward the correct payload to a `matchmaker` boundary, return `searching`, and pass the focused tests.
