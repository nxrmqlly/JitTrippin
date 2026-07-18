package engine

// Scheduler is responsible for bookeeping of Jobs in a pipeline.
// It stores standard bookeeping data for Topological sort using Kahn's algorithm
//
// Note: Not safe for concurrent use. It must be driven from single goroutine
//
// Internally: we use Kahn's Algorithm to schedule jobs in a pipeline
// based on dependencies. We execute all parent jobs (on which other
// jobs depend on) and then execute the next set of dependent
// processes and so on until all of them execute completely or return
// an error.
type Scheduler struct {
	jobMap   map[string]*Job
	indegree map[string]int
	children map[string][]string

	processed  int
	dispatched int
}

// NewScheduler Creates a new instance of a Scheduler and populates it with
// the jobs and their dependencies from a **validated** pipeline p.
func NewScheduler(p *Pipeline) *Scheduler {
	jobMap := make(map[string]*Job, len(p.Jobs))
	indegree := make(map[string]int, len(p.Jobs))
	children := make(map[string][]string)

	for i := range p.Jobs {
		job := &p.Jobs[i]

		jobMap[job.Name] = job
		indegree[job.Name] = len(job.DependsOn)

		for _, dep := range job.DependsOn {
			// we dont have to worry about duplicates because our validator
			// handles that for us. yay!
			children[dep] = append(children[dep], job.Name)
		}
	}

	return &Scheduler{
		jobMap:   jobMap,
		indegree: indegree,
		children: children,
	}
}

// Ready returns all available jobs ready to be executed now.
// (ie. Jobs with indegree = 0)
//
// Note: Ready should only be called once.
func (s *Scheduler) Ready() []*Job {
	var ready []*Job

	for name, job := range s.jobMap {
		if s.indegree[name] == 0 {
			ready = append(ready, job)
			s.dispatched++
		}
	}
	return ready
}

// Complete marks a jobName as complete and returns the next set of child Jobs
// that can be processed immediately
func (s *Scheduler) Complete(jobName string) []*Job {
	s.processed++

	var next []*Job

	for _, child := range s.children[jobName] {
		s.indegree[child]--

		if s.indegree[child] == 0 {
			next = append(next, s.jobMap[child])
			s.dispatched++
		}
	}

	return next
}

// Fail marks the jobName as processes but prevents any child jobs from
// becoming ready
func (s *Scheduler) Fail(jobName string) {
	s.processed++
}

// Done returns whether all jobs in the pipeline have finished executing.
func (s *Scheduler) Done() bool {
	return s.processed == s.dispatched
}
