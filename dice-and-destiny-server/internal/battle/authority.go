package battle

import (
	"encoding/json"
	"errors"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/engine"
)

type commandHandler interface {
	HandleCommand(cmd command.Command) engine.Result
}

// HandleCommand is the portable battle authority JSON boundary.
func HandleCommand(commandJSON string) string {
	return handleCommand(commandJSON, engine.NewEngine())
}

func handleCommand(commandJSON string, handler commandHandler) string {
	cmd, err := command.ParseJSON(commandJSON)
	if err != nil {
		return marshalResult(parseErrorResult(err))
	}

	return marshalResult(handler.HandleCommand(cmd))
}

func parseErrorResult(err error) engine.Result {
	switch {
	case errors.Is(err, command.ErrInvalidJSON):
		return engine.Result{
			Accepted: false,
			Error:    "invalid command JSON",
		}
	default:
		return engine.Result{
			Accepted: false,
			Error:    "invalid command envelope",
		}
	}
}

func marshalResult(r engine.Result) string {
	payload, err := json.Marshal(r)
	if err != nil {
		return `{"accepted":false,"error":"result serialization failed"}`
	}
	return string(payload)
}
