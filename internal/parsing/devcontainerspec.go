package parsing

import (
	"encoding/json"
	"errors"
	"fmt"
)

type Cmd struct {
	Array  []string
	String string
}

func (cmd *Cmd) UnmarshalJSON(b []byte) error {
	str := ""
	strErr := json.Unmarshal(b, &str)
	if strErr == nil {
		cmd.String = str
		return nil
	}
	arr := make([]string, 0)
	arrErr := json.Unmarshal(b, &arr)
	if arrErr == nil {
		cmd.Array = arr
		return nil
	}
	return fmt.Errorf("command cannot be parsed as string nor list: %w + %w", strErr, arrErr)
}

func (cmd Cmd) MarshalJSON() ([]byte, error) {
	if cmd.String != "" {
		return json.Marshal(cmd.String)
	}
	if cmd.Array != nil {
		return json.Marshal(cmd.Array)
	}
	return nil, errors.New("neither string nor array of cmd are defined, cannot marshal json")
}

type DevContainerSpec struct {
	Name    string   `json:"name"`
	Image   string   `json:"image,omitempty"`
	RunArgs []string `json:"runArgs,omitempty"`
	Build   struct {
		Dockerfile string            `json:"dockerfile"`
		Args       map[string]string `json:"args"`
	} `json:"build,omitempty"`
	Ports map[string]struct {
		Label string `json:"label"`
	} `json:"portsAttributes"`
	PostCreateCommand *Cmd `json:"postCreateCommand"`
	PostStartCommand  *Cmd `json:"postStartCommand"`
}
