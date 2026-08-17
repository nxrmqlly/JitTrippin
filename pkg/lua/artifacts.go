package lua

import (
	"github.com/nxrmqlly/jittrippin/pkg/artifact"
	lua "github.com/yuin/gopher-lua"
)

func luaArtifact(L *lua.LState) int {
	name := L.CheckString(1)
	path := L.CheckString(2)

	ud := L.NewUserData()
	ud.Value = &artifact.Artifact{Name: name, Path: path}
	L.Push(ud)
	return 1
}
