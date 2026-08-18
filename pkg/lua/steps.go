package lua

import (
	lua "github.com/yuin/gopher-lua"
)

type stepBuilder struct {
	name string
	cmd  string
}

func luaRun(L *lua.LState) int {
	nameOrCmd := L.CheckString(1)
	sb := &stepBuilder{}

	if L.GetTop() == 1 {
		// run "go test ./..."
		sb.cmd = nameOrCmd
	} else {
		// run "name" { cmd = "..." }
		sb.name = nameOrCmd
		parseStepConfig(L, L.CheckTable(2), sb)
	}

	ud := L.NewUserData()
	ud.Value = sb

	mt := L.NewTable()
	L.SetField(mt, "__call", L.NewFunction(stepCall))
	L.SetMetatable(ud, mt)

	L.Push(ud)
	return 1
}

func stepCall(L *lua.LState) int {
	ud := L.CheckUserData(1)
	sb := ud.Value.(*stepBuilder)

	parseStepConfig(L, L.CheckTable(2), sb)

	L.Push(ud)
	return 1
}

func parseStepConfig(L *lua.LState, cfg *lua.LTable, sb *stepBuilder) {
	cmd := cfg.RawGetString("cmd")
	if cmd == lua.LNil {
		L.ArgError(2, "missing 'cmd'")
		return
	}

	sb.cmd = lua.LVAsString(cmd)
}
