package parsing

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
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
	return nil, errors.New("Neither string nor array of cmd are defined, cannot marshal json")
}

type Feature struct {
	// TODO(juf): implement
	// Requires custom de/serialize for nicer handling
	Name    string
	Version string
	Hash    string
	Options map[string]any
}

type FeatureList struct {
	Features []Feature
}

func parseDependency(s string) (*Feature, error) {
	isVersion := false
	splitIdx := strings.Index(s, "@")
	if splitIdx == -1 {
		splitIdx = strings.Index(s, ":")
		if splitIdx == -1 {
			// TOOD(juf): Make this a concrete type
			return nil, fmt.Errorf("Dependency must specify hash or version tag, none give: %s", s)
		}
		isVersion = true
	}
	d := &Feature{}
	d.Name = s[0:splitIdx]
	versionOrHash := s[splitIdx+1:]
	if isVersion {
		d.Version = versionOrHash
	} else {
		d.Hash = versionOrHash
	}
	return d, nil

}

func (d *FeatureList) UnmarshalJSON(b []byte) error {
	raw := map[string]map[string]any{}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	for k, v := range raw {
		dep, err := parseDependency(k)
		if err != nil {
			return err
		}
		dep.Options = v
		d.Features = append(d.Features, *dep)
	}
	return nil
}

type FeatureSpec struct {
	ID               string         `json:"id"`      // Required
	Version          string         `json:"version"` // Required
	Name             string         `json:"name"`    // Required
	Description      string         `json:"description,omitempty"`
	DocumentationURL string         `json:"documentationURL,omitempty"`
	LicenseURL       string         `json:"licenseURL,omitempty"`
	Keywords         []string       `json:"keywords,omitempty"`
	Options          map[string]any `json:"options,omitempty"`
	ContainerEnv     map[string]any `json:"containerEnv,omitempty"`
	Privileged       bool           `json:"privileged,omitempty"`
	Init             bool           `json:"init,omitempty"`
	CapAdd           []string       `json:"capAdd,omitempty"`
	SecurityOpt      []string       `json:"securityOpt,omitempty"`
	Entrypoint       string         `json:"entrypoint,omitempty"`
	Customizations   map[string]any `json:"customizations,omitempty"`
	DependsOn        FeatureList    `json:"dependsOn,omitempty"`
	InstallsAfter    []string       `json:"installsAfter,omitempty"`
	LegacyIDs        []string       `json:"legacyIds,omitempty"`
	Deprecated       bool           `json:"deprecated,omitempty"`
	Mounts           map[string]any `json:"mounts,omitempty"`
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
	PostCreateCommand *Cmd        `json:"postCreateCommand"`
	PostStartCommand  *Cmd        `json:"postStartCommand"`
	Features          FeatureList `json:"features"`
}
