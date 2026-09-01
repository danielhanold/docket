package codexentry

import "encoding/json"

type rpcEnvelope struct {
	JSONRPC string          `json:"jsonrpc,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type threadStartResult struct {
	Thread struct {
		ID string `json:"id"`
	} `json:"thread"`
}

type turnStartResult struct {
	Turn struct {
		ID string `json:"id"`
	} `json:"turn"`
}

type completedItemParams struct {
	ThreadID string `json:"threadId"`
	TurnID   string `json:"turnId"`
	Item     struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"item"`
}

type completedTurnParams struct {
	ThreadID string `json:"threadId"`
	Turn     struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
		Items []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"items"`
	} `json:"turn"`
}
