package gostratum

import (
	"encoding/json"
	"errors"
)

// JsonRpcEvent represents a JSON-RPC event sent from the miner
type JsonRpcEvent struct {
	Id      any           `json:"id"` // id can be nil, a string, or an int
	Version string        `json:"jsonrpc"`
	Method  StratumMethod `json:"method"`
	Params  []any         `json:"params"`
}

// JsonRpcResponse represents a JSON-RPC response sent to the miner
type JsonRpcResponse struct {
	Id     any   `json:"id"`
	Result any   `json:"result"`
	Error  any   `json:"error"` // Changed to 'any' for better flexibility
}

// NewEvent creates a new JSON-RPC event with a given ID, method, and parameters
func NewEvent(id string, method string, params []any) JsonRpcEvent {
	var finalId any
	if len(id) == 0 {
		finalId = nil
	} else {
		finalId = id
	}

	return JsonRpcEvent{
		Id:      finalId,
		Version: "2.0",
		Method:  StratumMethod(method),
		Params:  params,
	}
}

// NewResponse creates a new JSON-RPC response based on an incoming event
func NewResponse(event JsonRpcEvent, results any, err any) JsonRpcResponse {
	return JsonRpcResponse{
		Id:     event.Id,
		Result: results,
		Error:  err,
	}
}

// UnmarshalEvent parses a JSON string into a JsonRpcEvent
func UnmarshalEvent(in string) (JsonRpcEvent, error) {
	event := JsonRpcEvent{}
	err := json.Unmarshal([]byte(in), &event)
	if err != nil {
		return JsonRpcEvent{}, errors.New("failed to parse JSON-RPC event: " + err.Error())
	}
	return event, nil
}

// UnmarshalResponse parses a JSON string into a JsonRpcResponse
func UnmarshalResponse(in string) (JsonRpcResponse, error) {
	resp := JsonRpcResponse{}
	err := json.Unmarshal([]byte(in), &resp)
	if err != nil {
		return JsonRpcResponse{}, errors.New("failed to parse JSON-RPC response: " + err.Error())
	}
	return resp, nil
}
