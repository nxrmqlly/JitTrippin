package lua

import (
	"github.com/nxrmqlly/jittrippin/pkg/engine"
	lua "github.com/yuin/gopher-lua"
)

func parseGithubConfig(tbl *lua.LTable) *engine.GitHub {
	gh := &engine.GitHub{}

	if v := tbl.RawGetString("push"); v != lua.LNil {
		if pushTbl, ok := v.(*lua.LTable); ok {
			pc := &engine.PushConfig{}
			if b := pushTbl.RawGetString("branch"); b != lua.LNil {
				pc.Branches = luaValueToStrings(b)
			}
			if t := pushTbl.RawGetString("tag"); t != lua.LNil {
				pc.Tags = luaValueToStrings(t)
			}
			gh.Push = pc
		}
	}

	if v := tbl.RawGetString("release"); v != lua.LNil {
		if relTbl, ok := v.(*lua.LTable); ok {
			rc := &engine.ReleaseConfig{}
			if o := relTbl.RawGetString("on"); o != lua.LNil {
				rc.On = lua.LVAsString(o)
			}
			if a := relTbl.RawGetString("artifacts"); a != lua.LNil {
				if artTbl, ok := a.(*lua.LTable); ok {
					rc.Artifacts = parseReleaseArtifacts(artTbl)
				}
			}
			gh.Release = rc
		}
	}

	return gh
}

func parseReleaseArtifacts(tbl *lua.LTable) []engine.ReleaseArtifact {
	var out []engine.ReleaseArtifact
	forEachArray(tbl, func(v lua.LValue) {
		if entry, ok := v.(*lua.LTable); ok {
			ra := engine.ReleaseArtifact{
				Job:  lua.LVAsString(entry.RawGetString("job")),
				Name: lua.LVAsString(entry.RawGetString("name")),
				As:   lua.LVAsString(entry.RawGetString("as")),
			}
			out = append(out, ra)
		}
	})
	return out
}
