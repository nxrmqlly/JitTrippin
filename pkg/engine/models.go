package engine

import (
	"github.com/nxrmqlly/jittrippin/pkg/artifact"
	"github.com/nxrmqlly/jittrippin/pkg/checkout"
)

type Pipeline struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Visibility  string            `json:"visibility,omitempty"`
	Checkout    checkout.Checkout `json:"checkout"`
	Jobs        []Job             `json:"jobs"`
	GitHub      *GitHub           `json:"github,omitempty"`
}

type GitHub struct {
	Push    *PushConfig    `json:"push,omitempty"`
	Release *ReleaseConfig `json:"release,omitempty"`
}

type PushConfig struct {
	Branches []string `json:"branches,omitempty"`
	Tags     []string `json:"tags,omitempty"`
}

type ReleaseConfig struct {
	On        string            `json:"on"`
	Artifacts []ReleaseArtifact `json:"artifacts"`
}

type ReleaseArtifact struct {
	Job  string `json:"job"`
	Name string `json:"name"`
	As   string `json:"as,omitempty"`
}

type Job struct {
	Name      string              `json:"name"`
	Image     string              `json:"image"`
	Steps     []Step              `json:"steps"`
	DependsOn []Dependency        `json:"depends_on"`
	Env       map[string]string   `json:"env"`
	Artifacts []artifact.Artifact `json:"artifacts,omitempty"`
}

type Dependency struct {
	Job      string   `json:"job"`
	Requires []string `json:"requires,omitempty"`
}

type Step struct {
	Name string `json:"name"`
	Cmd  string `json:"cmd"`
}
