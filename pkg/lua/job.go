package lua

import (
	"github.com/nxrmqlly/jittrippin/pkg/artifact"
	"github.com/nxrmqlly/jittrippin/pkg/engine"
	lua "github.com/yuin/gopher-lua"
)

type jobBuilder struct {
	name      string
	image     string
	env       map[string]string
	steps     []engine.Step
	dependsOn []engine.Dependency
	artifacts []artifact.Artifact
}

func luaJob(L *lua.LState) int {
	name := L.CheckString(1)

	jb := &jobBuilder{name: name}

	ud := L.NewUserData()
	ud.Value = jb

	mt := L.NewTable()
	L.SetField(mt, "__call", L.NewFunction(jobCall))
	L.SetMetatable(ud, mt)

	L.Push(ud)
	return 1
}

func jobCall(L *lua.LState) int {
	ud := L.CheckUserData(1)
	jb := ud.Value.(*jobBuilder)
	cfg := L.OptTable(2, nil)
	if cfg == nil {
		return 0
	}

	if v := cfg.RawGetString("image"); v != lua.LNil {
		jb.image = lua.LVAsString(v)
	}
	if v := cfg.RawGetString("env"); v != lua.LNil {
		if tbl, ok := v.(*lua.LTable); ok {
			jb.env = luaTableToStringMap(tbl)
		}
	}

	forEachArray(cfg, func(v lua.LValue) {
		if s, ok := unwrapStep(v); ok {
			jb.steps = append(jb.steps, *s)
		} else if d, ok := unwrapDependency(v); ok {
			jb.dependsOn = append(jb.dependsOn, *d)
		} else if a, ok := unwrapArtifact(v); ok {
			jb.artifacts = append(jb.artifacts, *a)
		}
	})
	L.Push(ud)
	return 1
}
