package lua

import (
	"github.com/nxrmqlly/jittrippin/pkg/engine"
	lua "github.com/yuin/gopher-lua"
)

func luaRun(L *lua.LState) int {
	nameOrCmd := L.CheckString(1)
	cfg := L.OptTable(2, nil)

	step := &engine.Step{}

	if cfg == nil {
		step.Cmd = nameOrCmd
	} else {
		step.Name = nameOrCmd

		cmd := cfg.RawGetString("cmd")
		if cmd == lua.LNil {
			L.ArgError(2, "missing 'cmd'")
			return 0
		}

		step.Cmd = lua.LVAsString(cmd)
	}

	ud := L.NewUserData()
	ud.Value = step

	L.Push(ud)
	return 1
}
