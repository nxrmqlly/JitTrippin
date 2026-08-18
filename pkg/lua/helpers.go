package lua

import (
	"github.com/nxrmqlly/jittrippin/pkg/artifact"
	"github.com/nxrmqlly/jittrippin/pkg/engine"
	lua "github.com/yuin/gopher-lua"
)

func luaTableToStringMap(tbl *lua.LTable) map[string]string {
	m := make(map[string]string)
	tbl.ForEach(func(key lua.LValue, val lua.LValue) {
		if k, ok := key.(lua.LString); ok {
			m[string(k)] = lua.LVAsString(val)
		}
	})
	return m
}

func luaTableToStrings(tbl *lua.LTable) []string {
	var s []string
	tbl.ForEach(func(_ lua.LValue, v lua.LValue) {
		if str, ok := v.(lua.LString); ok {
			s = append(s, string(str))
		}
	})
	return s
}

func luaValueToStrings(v lua.LValue) []string {
	switch val := v.(type) {
	case *lua.LTable:
		return luaTableToStrings(val)
	case lua.LString:
		return []string{string(val)}
	default:
		return []string{lua.LVAsString(v)}
	}
}

func forEachArray(tbl *lua.LTable, fn func(lua.LValue)) {
	var max int
	tbl.ForEach(func(k lua.LValue, _ lua.LValue) {
		if n, ok := k.(lua.LNumber); ok {
			i := int(n)
			if i > max {
				max = i
			}
		}
	})
	for i := 1; i <= max; i++ {
		v := tbl.RawGetInt(i)
		if v != lua.LNil {
			fn(v)
		}
	}
}

func unwrapStep(v lua.LValue) (*engine.Step, bool) {
	ud, ok := v.(*lua.LUserData)
	if !ok {
		return nil, false
	}
	sb, ok := ud.Value.(*stepBuilder)
	if !ok {
		return nil, false
	}
	step := &engine.Step{
		Name: sb.name,
		Cmd:  sb.cmd,
	}
	return step, true
}

func unwrapDependency(v lua.LValue) (*engine.Dependency, bool) {
	ud, ok := v.(*lua.LUserData)
	if !ok {
		return nil, false
	}
	s, ok := ud.Value.(*engine.Dependency)
	return s, ok

}

func unwrapArtifact(v lua.LValue) (*artifact.Artifact, bool) {
	ud, ok := v.(*lua.LUserData)
	if !ok {
		return nil, false
	}
	s, ok := ud.Value.(*artifact.Artifact)
	return s, ok
}
