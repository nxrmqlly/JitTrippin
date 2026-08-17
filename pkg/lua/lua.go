package lua

import (
	"fmt"
	"os"

	"github.com/nxrmqlly/jittrippin/pkg/engine"
	lua "github.com/yuin/gopher-lua"
)

func newVM() (*lua.LState, error) {
	L := lua.NewState(lua.Options{
		SkipOpenLibs: true,
	})

	lua.OpenBase(L)
	lua.OpenTable(L)
	lua.OpenMath(L)
	lua.OpenString(L)

	unsafe := []string{
		"loadfile", "dofile", "loadstring", "load",
		"rawget", "rawset", "rawequal", "collectgarbage",
	}
	for _, g := range unsafe {
		L.SetGlobal(g, lua.LNil)
	}

	return L, nil
}

func registerDSL(L *lua.LState, result **pipelineBuilder) {
	L.SetGlobal("pipeline", L.NewFunction(luaPipelineFunc(result)))
	L.SetGlobal("job", L.NewFunction(luaJob))
	L.SetGlobal("run", L.NewFunction(luaRun))
	L.SetGlobal("needs", L.NewFunction(luaNeeds))
	L.SetGlobal("artifact", L.NewFunction(luaArtifact))
}

func ProcessLua(data string) (*engine.Pipeline, error) {
	L, err := newVM()
	if err != nil {
		return nil, err
	}
	defer L.Close()

	var res *pipelineBuilder
	registerDSL(L, &res)
	if err := L.DoString(data); err != nil {
		return nil, err
	}
	if res == nil {
		return nil, fmt.Errorf("lua: no pipeline defined")
	}
	return buildPipeline(res)
}

func ProcessLuaFile(path string) (*engine.Pipeline, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ProcessLua(string(data))
}
