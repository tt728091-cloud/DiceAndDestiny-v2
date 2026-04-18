# Story 1: `lobby/registerclient`

## Story Goal

Create the `lobby` registration entry point so a client can register and receive a stable `client_id`.

This story owns only client registration. It does not request matches, assign sessions, or open websockets.

## Scope

- add `POST /v1/lobby/clients`
- validate the register request
- create a new `client_id`
- return the `registered` response
- add focused automated tests

## Exact Contract

### Request

`POST /v1/lobby/clients`

```json
{
  "display_name": "test-client-1"
}
```

### Success Response

`201 Created`

```json
{
  "client_id": "client-1",
  "status": "registered"
}
```

## Acceptance Criteria

- valid register requests are accepted
- a stable `client_id` is generated and returned
- response shape matches the contract
- invalid register requests are rejected cleanly
- focused automated tests pass

## Tests Required

- Go test for request validation
- Go test for response shape
- Go test proving a generated `client_id` is returned

## Touched Paths

```text
v2/server/lobby/internal/app/registerclient/
v2/server/lobby/internal/transport/http/
```

## Rules And Constraints

- server only
- do not implement match request behavior here
- keep ID generation injectable if needed for deterministic tests
- keep comments short and useful where boundaries or intent are not obvious
- V1 reuse is allowed only if the reused code fits this exact story cleanly

## Out Of Scope

- `requestmatch`
- `lookupassignment`
- websocket logic
- matchmaker integration

## Assumptions For Now

- `display_name` is required and should be non-empty
- exact display name validation can stay minimal in this story
- registration state may be in-memory for the first milestone if hidden behind a clean boundary

## Completion Proof

This story is complete when the endpoint exists, the contract matches, and the focused tests pass.
