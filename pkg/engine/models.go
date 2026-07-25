package engine

import "github.com/nxrmqlly/jittrippin/pkg/artifact"

type Pipeline struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Jobs        []Job  `json:"jobs"`
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

type JobResult struct {
	job *Job
	err error
}
