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

// https://containers.dev/implementors/json_reference/
type DevContainerSpec struct {
	// A name for the dev container displayed in the UI.
	Name string `json:"name"`
	// The name of an image in a container registry.
	Image string `json:"image,omitempty"`
	// An array of arguments that should be used when running the container.
	RunArgs []string `json:"runArgs,omitempty"`
	Build   struct {
		// The location of a Dockerfile that defines the contents of the container.
		Dockerfile string `json:"dockerfile"`
		// A set of name-value pairs containing Docker image build arguments that should be passed when building a Dockerfile.
		Args map[string]string `json:"args"`
	} `json:"build,omitempty"`
	Ports map[string]struct {
		// Display name for the port in the ports view.
		Label string `json:"label"`
	} `json:"portsAttributes"`
	// This command is the last of three that finalizes container setup when a dev container is created.
	PostCreateCommand *Cmd `json:"postCreateCommand"`
	// A command to run each time the container is successfully started.
	PostStartCommand *Cmd `json:"postStartCommand"`
	// Add additional mounts to a container.
	Mounts []*Mount `json:"mounts,omitempty"`
}

// https://docs.docker.com/engine/storage/bind-mounts/#options-for---mount
type Mount struct {
	// Valid options for type include:
	// `bind` mounts an existing persistentVolumeClaim
	// `volume` automatically creates the persistentVolumeClaim that does not yet exist and mounts it.
	Type string `json:"type,omitempty"`
	// The persistentVolumeClaim name.
	Source string `json:"source,omitempty"`
	// The path where the file or directory is mounted in the container. Must be an absolute path.
	Target string `json:"target,omitempty"`
	// If true, causes the bind mount to be mounted into the container as read-only. Default false.
	Readonly bool `json:"readonly,omitempty"`
}

// https://docs.docker.com/engine/storage/bind-mounts/#options-for---mount
func (m *Mount) UnmarshalJSON(data []byte) error {
	if string(data) == "null" || string(data) == `""` {
		return nil
	}
	var j interface{}
	if err := json.Unmarshal(data, &j); err != nil {
		return err
	}

	if r, ok := j.(map[string]interface{}); ok {
		if t, ok := r["type"].(string); ok {
			m.Type = t
		}

		if source, ok := r["source"].(string); ok {
			m.Source = source
		} else if src, ok := r["src"].(string); ok {
			m.Source = src
		}

		if target, ok := r["target"].(string); ok {
			m.Target = target
		} else if destination, ok := r["destination"].(string); ok {
			m.Target = destination
		} else if dst, ok := r["dst"].(string); ok {
			m.Target = dst
		}

		if readonly, ok := r["readonly"].(bool); ok {
			m.Readonly = readonly
		} else if ro, ok := r["ro"].(bool); ok {
			m.Readonly = ro
		} else {
			m.Readonly = false
		}
	}

	return nil
}
