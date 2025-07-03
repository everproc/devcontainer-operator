package parsing

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/go-containerregistry/pkg/name"
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

type FeatureRequest struct {
	// TODO(juf): implement
	// Requires custom de/serialize for nicer handling
	Options map[string]any
	Ref     name.Reference
}

type FeatureList struct {
	Features []FeatureRequest
}

func ParseDependency(s string) (*FeatureRequest, error) {
	ref, err := name.ParseReference(s)
	if err != nil {
		return nil, err
	}
	d := &FeatureRequest{
		Options: make(map[string]any),
		Ref:     ref,
	}
	return d, nil

}

func (d *FeatureList) UnmarshalJSON(b []byte) error {
	raw := map[string]map[string]any{}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	for k, v := range raw {
		dep, err := ParseDependency(k)
		if err != nil {
			return err
		}
		dep.Options = v
		d.Features = append(d.Features, *dep)
	}
	return nil
}

type FeatureSpecOption struct {
	ID          string
	Type        string
	Description string
	Proposals   []string
	Default     string
}

type FeatureSpecOptionList struct {
	Options map[string]FeatureSpecOption
}

func (o *FeatureSpecOptionList) UnmarshalJSON(b []byte) error {
	raw := map[string]map[string]any{}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	for k, v := range raw {
		var proposals []string = nil
		if p, ok := v["proposals"]; ok {
			for _, pv := range p.([]any) {
				proposals = append(proposals, fmt.Sprintf("%s", pv))
			}
		}
		defaultVal := ""
		switch v["default"].(type) {
		case string:
			defaultVal = v["default"].(string)
		default:
			// TODO(juf): fix this
			b, err := json.Marshal(v["default"])
			if err != nil {
				panic(err)
			}
			defaultVal = string(b)
		}
		opt := FeatureSpecOption{
			ID:          k,
			Type:        v["type"].(string),
			Description: v["description"].(string),
			Proposals:   proposals,
			Default:     defaultVal,
		}
		if o.Options == nil {
			o.Options = make(map[string]FeatureSpecOption)
		}
		o.Options[k] = opt
	}
	return nil
}

type FeatureSpec struct {
	ID               string                `json:"id"`      // Required
	Version          string                `json:"version"` // Required
	Name             string                `json:"name"`    // Required
	Description      string                `json:"description,omitempty"`
	DocumentationURL string                `json:"documentationURL,omitempty"`
	LicenseURL       string                `json:"licenseURL,omitempty"`
	Keywords         []string              `json:"keywords,omitempty"`
	Options          FeatureSpecOptionList `json:"options"`
	ContainerEnv     map[string]any        `json:"containerEnv,omitempty"`
	Privileged       bool                  `json:"privileged,omitempty"`
	Init             bool                  `json:"init,omitempty"`
	CapAdd           []string              `json:"capAdd,omitempty"`
	SecurityOpt      []string              `json:"securityOpt,omitempty"`
	Entrypoint       string                `json:"entrypoint,omitempty"`
	Customizations   map[string]any        `json:"customizations,omitempty"`
	DependsOn        FeatureList           `json:"dependsOn"`
	InstallsAfter    []string              `json:"installsAfter,omitempty"`
	LegacyIDs        []string              `json:"legacyIds,omitempty"`
	Deprecated       bool                  `json:"deprecated,omitempty"`
	Mounts           []*Mount              `json:"mounts,omitempty"`
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
	} `json:"build"`
	Ports map[string]struct {
		// Display name for the port in the ports view.
		Label string `json:"label"`
	} `json:"portsAttributes"`
	// Specifies the username to use when connecting to the remote development environment.
	RemoteUser string `json:"remoteUser"`
	// Specifies the username to use inside the container during development.
	ContainerUser string `json:"containerUser"`
	// This command is the last of three that finalizes container setup when a dev container is created.
	PostCreateCommand *Cmd `json:"postCreateCommand"`
	// A command to run each time the container is successfully started.
	PostStartCommand *Cmd `json:"postStartCommand"`
	// Add additional mounts to a container.
	Mounts []*Mount `json:"mounts,omitempty"`
	// Augment devcontainer with a list of features
	Features FeatureList `json:"features"`
}

// https://docs.docker.com/engine/storage/bind-mounts/#options-for---mount
type Mount struct {
	// Valid options for type include:
	// `bind` mounts an existing persistentVolumeClaim
	// `volume` automatically creates the persistentVolumeClaim that does not yet exist and mounts it.
	Type string `json:"type,omitempty"`
	// The persistentVolumeClaim label src=<Source>.
	Source string `json:"source,omitempty"`
	// The path where the file or directory is mounted in the container. Must be an absolute path.
	Target string `json:"target,omitempty"`
	// If true, causes the bind mount to be mounted into the container as read-only. Default false.
	Readonly bool `json:"readonly,omitempty"`
}

// https://docs.docker.com/engine/storage/bind-mounts/#options-for---mount
func (m *Mount) UnmarshalJSON(data []byte) error {
	if data == nil || string(data) == "null" || string(data) == `""` {
		return nil
	}
	var j any
	if err := json.Unmarshal(data, &j); err != nil {
		return err
	}

	if r, ok := j.(map[string]any); ok {
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
