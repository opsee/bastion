package messaging

type Command struct {
	Action     string                 `json:"action"`
	Parameters map[string]interface{} `json:"parameters"`
}
