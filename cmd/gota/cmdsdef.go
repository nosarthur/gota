package main

import (
	_ "embed"
	"encoding/json"
	"os"
	"sync"
)

//go:embed cmds.json
var defaultCmdsJSON []byte

type CmdDef struct {
	Cmd          string `json:"cmd"`
	Help         string `json:"help"`
	AllowAll     bool   `json:"allow_all"`
	DisableAsync bool   `json:"disable_async"`
	Shell        bool   `json:"shell"`
}

var (
	cmdsOnce sync.Once
	cmdsMap  map[string]CmdDef
)

// getCmds: delegated git sub-commands; user cmds.json shadows defaults per key.
func getCmds() map[string]CmdDef {
	cmdsOnce.Do(func() {
		cmdsMap = map[string]CmdDef{}
		json.Unmarshal(defaultCmdsJSON, &cmdsMap)
		data, err := os.ReadFile(configFname("cmds.json"))
		if err == nil && len(data) > 0 {
			custom := map[string]CmdDef{}
			if json.Unmarshal(data, &custom) == nil {
				for k, v := range custom {
					cmdsMap[k] = v
				}
			}
		}
	})
	return cmdsMap
}

// asyncBlacklist: gita cmd names with disable_async (matched against cmd[1],
// same quirk as upstream)
func asyncBlacklist() map[string]bool {
	bl := map[string]bool{}
	for name, def := range getCmds() {
		if def.DisableAsync {
			bl[name] = true
		}
	}
	return bl
}
