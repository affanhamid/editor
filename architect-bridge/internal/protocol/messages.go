package protocol

import "encoding/json"

type Event struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

type Command struct {
	Type    string          `json:"type"`
	RawData json.RawMessage `json:"data"`
	Conn    *ClientConn     `json:"-"`
}

func (c *Command) DataString(key string) (string, bool) {
	var m map[string]interface{}
	if err := json.Unmarshal(c.RawData, &m); err != nil {
		return "", false
	}
	v, ok := m[key].(string)
	return v, ok
}

func (c *Command) DataInt(key string) (int, bool) {
	var m map[string]interface{}
	if err := json.Unmarshal(c.RawData, &m); err != nil {
		return 0, false
	}
	v, ok := m[key].(float64)
	return int(v), ok
}

func (c *Command) DataBool(key string) (bool, bool) {
	var m map[string]interface{}
	if err := json.Unmarshal(c.RawData, &m); err != nil {
		return false, false
	}
	v, ok := m[key].(bool)
	return v, ok
}
