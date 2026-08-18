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

func luaPipeline(result **pipelineBuilder) lua.LGFunction {
	return func(L *lua.LState) int {
		if *result != nil {
			L.RaiseError("pipeline: only one pipeline may be defined")
			return 0
		}
		
		name := L.CheckString(1)
		pb := &pipelineBuilder{
			name: name,
		}
		
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
		cfg := L.OptTable(2, nil)

		if cfg != nil {
			if v := cfg.RawGetString("description"); v != lua.LNil {
				pb.description = lua.LVAsString(v)
			}
			if v := cfg.RawGetString("visibility"); v != lua.LNil {
				pb.visibility = lua.LVAsString(v)
			}
		}
		*result = pb
		return 0
	}
}

func luaCheckout(result **pipelineBuilder) lua.LGFunction {
	return func(L *lua.LState) int {
		if *result == nil {
			L.RaiseError("checkout: pipeline must be declared first")
			return 0
		}
		cfg := L.CheckTable(1)
		if u := cfg.RawGetString("url"); u != lua.LNil {
			(*result).checkout.URL = lua.LVAsString(u)
		}
		if b := cfg.RawGetString("branch"); b != lua.LNil {
			(*result).checkout.Ref = lua.LVAsString(b)
		}
		return 0
	}
}

func luaGithub(result **pipelineBuilder) lua.LGFunction {
	return func(L *lua.LState) int {
		if *result == nil {
			L.RaiseError("github: pipeline must be declared first")
			return 0
		}

		cfg := L.CheckTable(1)
		(*result).github = parseGithubConfig(cfg)

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
