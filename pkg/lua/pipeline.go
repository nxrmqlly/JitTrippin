package lua

import (
	"github.com/nxrmqlly/jittrippin/pkg/artifact"
	"github.com/nxrmqlly/jittrippin/pkg/checkout"
	"github.com/nxrmqlly/jittrippin/pkg/engine"
	lua "github.com/yuin/gopher-lua"
)

type pipelineBuilder struct {
	name        string
	description string
	visibility  string
	checkout    checkout.Checkout
	jobs        []*jobBuilder
	github      *engine.GitHub
}

func luaPipelineFunc(result **pipelineBuilder) lua.LGFunction {
	return func(L *lua.LState) int {
		name := L.CheckString(1)
		pb := &pipelineBuilder{name: name}

		ud := L.NewUserData()
		ud.Value = pb

		mt := L.NewTable()
		L.SetField(mt, "__call", L.NewFunction(pipelineCallFunc(result, pb)))
		L.SetMetatable(ud, mt)

		L.Push(ud)
		return 1
	}
}

func pipelineCallFunc(result **pipelineBuilder, pb *pipelineBuilder) lua.LGFunction {
	return func(L *lua.LState) int {
		ud := L.CheckUserData(1)
		_ = ud.Value.(*pipelineBuilder) // self
		cfg := L.OptTable(2, nil)

		if cfg != nil {
			if v := cfg.RawGetString("description"); v != lua.LNil {
				pb.description = lua.LVAsString(v)
			}
			if v := cfg.RawGetString("visibility"); v != lua.LNil {
				pb.visibility = lua.LVAsString(v)
			}
			if v := cfg.RawGetString("checkout"); v != lua.LNil {
				if tbl, ok := v.(*lua.LTable); ok {
					if u := tbl.RawGetString("url"); u != lua.LNil {
						pb.checkout.URL = lua.LVAsString(u)
					}
					if b := tbl.RawGetString("branch"); b != lua.LNil {
						pb.checkout.Ref = lua.LVAsString(b)
					}
				}
			}
			if v := cfg.RawGetString("github"); v != lua.LNil {
				if tbl, ok := v.(*lua.LTable); ok {
					pb.github = parseGithubConfig(tbl)
				}
			}
			forEachArray(cfg, func(v lua.LValue) {
				if jb, ok := unwrapJobBuilder(v); ok {
					pb.jobs = append(pb.jobs, jb)
				}
			})
		}
		*result = pb
		return 0
	}
}

func buildPipeline(pb *pipelineBuilder) (*engine.Pipeline, error) {
	p := &engine.Pipeline{
		Name:        pb.name,
		Description: pb.description,
		Visibility:  pb.visibility,
		Checkout:    pb.checkout,
		GitHub:      pb.github,
	}

	for _, jb := range pb.jobs {
		job := engine.Job{
			Name:      jb.name,
			Image:     jb.image,
			Steps:     jb.steps,
			DependsOn: jb.dependsOn,
			Env:       jb.env,
		}
		if len(jb.artifacts) > 0 {
			job.Artifacts = make([]artifact.Artifact, len(jb.artifacts))
			copy(job.Artifacts, jb.artifacts)
		}
		p.Jobs = append(p.Jobs, job)
	}
	return p, nil
}
