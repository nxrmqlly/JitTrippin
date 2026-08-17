package engine

import (
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

var re = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

type ValidationError struct {
	Location string
	Message  string
}

type ValidationErrors struct {
	Errors []ValidationError
}

// String  generates the error string for a validation error
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
	p.validateArtifacts(&errs)
	p.validateGraphs(&errs)
	p.validateGitHub(&errs)

	if len(errs.Errors) > 0 {
		return errs
	}

	return nil
}

func (p *Pipeline) validateStepFields(job Job, errs *ValidationErrors) {
	for sIdx, s := range job.Steps {

		jobStep := fmt.Sprintf("step %d '%s/%s'", sIdx+1, job.Name, s.Name)

		if s.Cmd == "" {
			errs.Add(ValidationError{
				Location: jobStep,
				Message:  "cmd cannot be empty",
			})
		}
	}
}

func (p *Pipeline) validateArtifactFields(job Job, errs *ValidationErrors) {
	artifactNames := make(map[string]struct{})
	for _, a := range job.Artifacts {

		jobArtifact := fmt.Sprintf("artifact '%s/%s'", job.Name, a.Name)

		if !re.MatchString(a.Name) {
			errs.Add(ValidationError{
				Location: jobArtifact,
				Message:  "name must be a valid identifier",
			})
		}

		if _, exists := artifactNames[a.Name]; exists {
			errs.Add(ValidationError{
				Location: jobArtifact,
				Message:  "duplicate artifact name",
			})
		}

		artifactNames[a.Name] = struct{}{}

		if a.Path == "" {
			errs.Add(ValidationError{
				Location: jobArtifact,
				Message:  "path cannot be empty",
			})
		}
	}
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

	switch p.Visibility {
	case "", "private", "public":
	default:
		errs.Add(ValidationError{
			Location: pipelineLocation,
			Message:  "visibility must be 'public' or 'private'",
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

		if !re.MatchString(job.Name) {
			errs.Add(ValidationError{
				Location: jobLocation,
				Message:  "name must be a valid identifier",
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

		p.validateStepFields(job, errs)
		p.validateArtifactFields(job, errs)

	}
}

// validateDependencies checks if the dependency nodes exist
func (p *Pipeline) validateDependencies(errs *ValidationErrors) {
	jobs := make(map[string]struct{})

	jobArtifacts := make(map[string]map[string]struct{})

	for _, job := range p.Jobs {
		jobs[job.Name] = struct{}{}
		names := make(map[string]struct{})
		for _, a := range job.Artifacts {
			names[a.Name] = struct{}{}
		}
		jobArtifacts[job.Name] = names
	}

	for idx, job := range p.Jobs {
		seen := make(map[string]struct{})

		for _, dep := range job.DependsOn {

			jobLocation := fmt.Sprintf("job %d '%s'", idx+1, job.Name)

			if !re.MatchString(dep.Job) {
				errs.Add(ValidationError{
					Location: jobLocation,
					Message:  "name must be a valid identifier",
				})
			}

			if dep.Job == job.Name {
				errs.Add(ValidationError{
					Location: jobLocation,
					Message:  "job cannot depend on itself",
				})
			}

			if _, ok := jobs[dep.Job]; !ok {
				errs.Add(ValidationError{
					Location: jobLocation,
					Message:  fmt.Sprintf("dependency '%s' does not exist", dep.Job),
				})
			}

			if _, ok := seen[dep.Job]; ok {
				errs.Add(ValidationError{
					Location: jobLocation,
					Message:  fmt.Sprintf("duplicate dependency '%s'", dep.Job),
				})
			}

			seen[dep.Job] = struct{}{}

			depArtSeen := make(map[string]struct{})
			for _, aName := range dep.Requires {
				if !re.MatchString(aName) {
					errs.Add(ValidationError{
						Location: jobLocation,
						Message:  "name must be a valid identifier",
					})
				}

				if _, ok := depArtSeen[aName]; ok {
					errs.Add(ValidationError{
						Location: jobLocation,
						Message:  fmt.Sprintf("dependency '%s' references artifact '%s' more than once", dep.Job, aName),
					})
				}

				if targets, ok := jobArtifacts[dep.Job]; ok {
					if _, exists := targets[aName]; !exists {
						errs.Add(ValidationError{
							Location: jobLocation,
							Message:  fmt.Sprintf("dependency '%s' does not produce artifact '%s'", dep.Job, aName),
						})
					}
				}
			}
		}
	}
}

func (p *Pipeline) validateArtifacts(errs *ValidationErrors) {
	for _, job := range p.Jobs {
		for _, a := range job.Artifacts {
			jobArtifact := fmt.Sprintf("artifact '%s/%s'", job.Name, a.Name)

			if filepath.IsAbs(a.Path) {
				errs.Add(ValidationError{
					Location: jobArtifact,
					Message:  "path must not be absolute",
				})
			}

			clean := filepath.Clean(a.Path)
			if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
				errs.Add(ValidationError{
					Location: jobArtifact,
					Message:  "path cannot go back directories",
				})
			}
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
			if _, exists := jobMap[dep.Job]; !exists {
				continue
			}

			switch color[dep.Job] {
			case white:
				dfs(dep.Job)
			case gray:
				cycleStart := pathIndex[dep.Job]
				cycle := append(append([]string{}, path[cycleStart:]...), dep.Job)
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

func (p *Pipeline) validateGitHub(errs *ValidationErrors) {
	if p.GitHub == nil {
		return
	}

	if pc := p.GitHub.Push; pc != nil {
		for _, g := range append(append([]string{}, pc.Branches...), pc.Tags...) {
			if _, err := path.Match(g, "sample"); err != nil {
				errs.Add(ValidationError{
					Location: "github.push",
					Message:  "branches/tags must be valid glob patterns: " + err.Error(),
				})
			}
		}
	}

	rel := p.GitHub.Release
	if rel == nil {
		return
	}

	switch rel.On {
	case "", "created", "edited", "published", "prereleased", "released":
	default:
		errs.Add(ValidationError{
			Location: "github.release.on",
			Message:  "must be a github release action: created, edited, published, prereleased, released",
		})
	}

	seen := make(map[string]struct{})
	for idx, a := range rel.Artifacts {
		location := fmt.Sprintf("release artifact %d", idx+1)
		if a.Job == "" || a.Name == "" {
			errs.Add(ValidationError{Location: location, Message: "job and name are required"})
			continue
		}
		if _, err := p.LookupArtifact(a.Job, a.Name); err != nil {
			errs.Add(ValidationError{Location: location, Message: err.Error()})
		}
		key := a.Job + "/" + a.Name
		if _, ok := seen[key]; ok {
			errs.Add(ValidationError{Location: location, Message: fmt.Sprintf("duplicate release artifact '%s'", key)})
		}
		seen[key] = struct{}{}
		if a.As != "" && !re.MatchString(a.As) {
			errs.Add(ValidationError{Location: location, Message: "as must be a valid filename"})
		}
	}
}
