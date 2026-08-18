package lua

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/nxrmqlly/jittrippin/pkg/engine"
	lua "github.com/yuin/gopher-lua"
)

func newVM(ctx context.Context) (*lua.LState, error) {
	L := lua.NewState(lua.Options{
		SkipOpenLibs:  true,
		CallStackSize: 128,

		RegistrySize:     1024,
		RegistryMaxSize:  8192,
		RegistryGrowStep: 1024,
	})
	L.SetContext(ctx)
	L.SetMx(32) // 32 MiB max

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
	L.SetGlobal("pipeline", L.NewFunction(luaPipeline(result)))
	L.SetGlobal("checkout", L.NewFunction(luaCheckout(result)))
	L.SetGlobal("job", L.NewFunction(luaJob(result)))
	L.SetGlobal("github", L.NewFunction(luaGithub(result)))

	L.SetGlobal("run", L.NewFunction(luaRun))
	L.SetGlobal("needs", L.NewFunction(luaNeeds))
	L.SetGlobal("artifact", L.NewFunction(luaArtifact))
}

func ProcessLua(ctx context.Context, data string) (*engine.Pipeline, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	L, err := newVM(ctx)
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

func ProcessLuaFile(ctx context.Context, path string) (*engine.Pipeline, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ProcessLua(ctx, string(data))
}
