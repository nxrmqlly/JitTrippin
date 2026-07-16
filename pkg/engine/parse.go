package engine

import (
	"encoding/json"
	"os"
)

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

	var pipeline Pipeline

	decoder := json.NewDecoder(file)
	decoder.Decode(&pipeline)

	return &pipeline, nil
}
