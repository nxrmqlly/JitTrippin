package lua

import (
	"github.com/nxrmqlly/jittrippin/pkg/engine"
	lua "github.com/yuin/gopher-lua"
)

func luaNeeds(L *lua.LState) int {
	job := L.CheckString(1)
	cfg := L.OptTable(2, nil)

	dep := &engine.Dependency{Job: job}

	if cfg != nil {
		if v := cfg.RawGetString("requires"); v != lua.LNil {
			if tbl, ok := v.(*lua.LTable); ok {
				dep.Requires = luaTableToStrings(tbl)
			}
		}
	}
	ud := L.NewUserData()
	ud.Value = dep
	L.Push(ud)
	return 1
}
