package engine

import (
	"fmt"
	"strings"
)

type ValidationError struct {
	Location string
	Message  string
}

type ValidationErrors struct {
	Errors []ValidationError
}

// Error generates the error string for a validation error
func (v ValidationError) String() string {
	return v.Location + ": " + v.Message
}

// Error generates the error string summary for all validation errors
//
// It returns a bulleted list.
func (e ValidationErrors) Error() string {
	var b strings.Builder

	fmt.Fprintf(&b, "pipeline validation failed (%d errors)", len(e.Errors))
	for _, err := range e.Errors {
		fmt.Fprintf(&b, "\n - %s", err.String())
	}

	return b.String()
}

// Add appends a ValidationError to ValidationErrors.Errors
func (e *ValidationErrors) Add(err ValidationError) {
	e.Errors = append(e.Errors, err)
}

func (p *Pipeline) Validate() error {
	var errs ValidationErrors

	p.validateFields(&errs)
	p.validateDependencies(&errs)
	p.validateGraphs(&errs)

	if len(errs.Errors) > 0 {
		return &errs
	}

	return nil
}

// validateFields checks if required fields are non-empty
func (p *Pipeline) validateFields(errs *ValidationErrors) {

	pipelineLocation := fmt.Sprintf("pipeline '%s'", p.Name)
	if p.Name == "" {
		errs.Add(ValidationError{
			Location: pipelineLocation,
			Message:  "name cannot be empty",
		})
	}

	if len(p.Jobs) == 0 {
		errs.Add(ValidationError{
			Location: pipelineLocation,
			Message:  "must contain at least one job",
		})
	}

	jobNames := make(map[string]struct{})

	for idx, job := range p.Jobs {

		jobLocation := fmt.Sprintf("job %d '%s'", idx+1, job.Name)

		if job.Name == "" {
			errs.Add(ValidationError{
				Location: jobLocation,
				Message:  "name cannot be empty",
			})
		}

		if _, exists := jobNames[job.Name]; exists {
			errs.Add(ValidationError{
				Location: jobLocation,
				Message:  "duplicate job name",
			})
		}

		jobNames[job.Name] = struct{}{}

		if job.Image == "" {
			errs.Add(ValidationError{
				Location: jobLocation,
				Message:  "image cannot be empty",
			})
		}

		if len(job.Steps) == 0 {
			errs.Add(ValidationError{
				Location: jobLocation,
				Message:  "must have at least one step",
			})
		}

		stepNames := make(map[string]struct{})

		for sIdx, step := range job.Steps {

			jobStep := fmt.Sprintf("step %d '%s/%s'", sIdx+1, job.Name, step.Name)

			if step.Name == "" {
				errs.Add(ValidationError{
					Location: jobStep,
					Message:  "name cannot be empty",
				})
			}

			if _, exists := stepNames[step.Name]; exists {
				errs.Add(ValidationError{
					Location: jobStep,
					Message:  "duplicate step name",
				})
			}

			stepNames[step.Name] = struct{}{}

			if step.Cmd == "" {
				errs.Add(ValidationError{
					Location: jobStep,
					Message:  "cmd cannot be empty",
				})
			}
		}
	}
}

// validateDependencies checks if the dependency nodes exist
func (p *Pipeline) validateDependencies(errs *ValidationErrors) {
	jobs := make(map[string]struct{})

	for _, job := range p.Jobs {
		jobs[job.Name] = struct{}{}
	}

	for idx, job := range p.Jobs {
		seen := make(map[string]struct{})

		for _, dep := range job.DependsOn {

			jobLocation := fmt.Sprintf("job %d '%s'", idx+1, job.Name)

			if dep == job.Name {
				errs.Add(ValidationError{
					Location: jobLocation,
					Message:  "job cannot depend on itself",
				})
			}

			if _, ok := jobs[dep]; !ok {
				errs.Add(ValidationError{
					Location: jobLocation,
					Message:  fmt.Sprintf("dependency '%s' does not exist", dep),
				})
			}

			if _, ok := seen[dep]; ok {
				errs.Add(ValidationError{
					Location: jobLocation,
					Message:  fmt.Sprintf("duplicate dependency '%s'", dep),
				})
			}

			seen[dep] = struct{}{}
		}
	}
}

func (p *Pipeline) validateGraphs(errs *ValidationErrors) {
	jobMap := make(map[string]Job, len(p.Jobs))
	for _, job := range p.Jobs {
		jobMap[job.Name] = job
	}

	const (
		white = iota // unvisited
		gray         // on current DFS path
		black        // fully processed
	)

	color := make(map[string]int, len(p.Jobs))
	var path []string
	pathIndex := make(map[string]int)

	var dfs func(string)
	dfs = func(name string) {
		color[name] = gray

		pathIndex[name] = len(path) // before we add next el
		path = append(path, name)

		for _, dep := range jobMap[name].DependsOn {
			if _, exists := jobMap[dep]; !exists {
				continue
			}

			switch color[dep] {
			case white:
				dfs(dep)
			case gray:
				cycleStart := pathIndex[dep]
				cycle := append(append([]string{}, path[cycleStart:]...), dep)
				errs.Add(ValidationError{
					Location: fmt.Sprintf("job '%s'", name),
					Message:  fmt.Sprintf("dependency cycle detected: %s", strings.Join(cycle, " -> ")),
				})
			}
		}

		delete(pathIndex, name)
		path = path[:len(path)-1]
		color[name] = black
	}

	for _, job := range p.Jobs {
		if color[job.Name] == white {
			dfs(job.Name)
		}
	}
}
