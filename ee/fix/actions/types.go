package actions

type actionState string

const (
	actionUnknown actionState = "unknown"
	actionNoop                = "noop"
	actionSuccess             = "success"
	actionFailure             = "failure"
)

type actionInput struct {
	PriorState  actionState
	PriorReason string
	Data        map[string]any
}

type actionOutput struct {
	State  actionState
	Reason string
	Data   map[string]any
}

func NewActionInput() actionInput {
	return actionInput{
		Data: make(map[string]any),
	}
}

func NewActionOutput() actionOutput {
	return actionOutput{
		Data: make(map[string]any),
	}
}
