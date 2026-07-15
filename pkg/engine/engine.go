package engine

import (
	"encoding/json"
	"os"
)

type Pipeline struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Jobs        []Job  `json:"jobs"`
}

type Job struct {
	Name      string            `json:"name"`
	Image     string            `json:"image"`
	Steps     []Step            `json:"steps"`
	DependsOn []string          `json:"depends_on"`
	Env       map[string]string `json:"env"`
}

type Step struct {
	Name string `json:"name"`
	Cmd  string `json:"cmd"`
}

func ProcessJSON(data string) (*Pipeline, error) {
	var pipeline Pipeline
	err := json.Unmarshal([]byte(data), &pipeline)

	if err != nil {
		return nil, err
	}

    return &pipeline, nil
}

func ProcessJSONFile(path string) (*Pipeline, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()



	return nil, nil
}
