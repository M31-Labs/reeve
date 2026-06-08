package workspaces

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

type Workspace struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type Registry map[string]Workspace

func Load(ctx context.Context, command string) (Registry, []string, error) {
	reg := Registry{}
	if strings.TrimSpace(command) == "" {
		return reg, []string{"graft command is empty; workspace join skipped"}, nil
	}
	args := splitCommand(command, "workspace", "list", "--json")
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return reg, []string{fmt.Sprintf("%s not found; workspace join skipped", args[0])}, nil
		}
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return reg, []string{fmt.Sprintf("graft workspace list failed: %s", msg)}, nil
	}
	workspaces, err := ParseJSON(out)
	if err != nil {
		return reg, nil, err
	}
	for _, ws := range workspaces {
		if ws.Name == "" || ws.Path == "" {
			continue
		}
		reg[ws.Name] = ws
	}
	return reg, nil, nil
}

func ParseJSON(body []byte) ([]Workspace, error) {
	var root any
	if err := json.Unmarshal(body, &root); err != nil {
		return nil, err
	}
	var out []Workspace
	collect(root, &out)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (r Registry) Find(name string) (Workspace, bool) {
	ws, ok := r[name]
	return ws, ok
}

func ShortName(spaceID string) string {
	if i := strings.LastIndex(spaceID, "/"); i >= 0 && i+1 < len(spaceID) {
		return spaceID[i+1:]
	}
	return spaceID
}

func splitCommand(command string, extra ...string) []string {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		parts = []string{command}
	}
	return append(parts, extra...)
}

func collect(v any, out *[]Workspace) {
	switch x := v.(type) {
	case []any:
		for _, item := range x {
			collect(item, out)
		}
	case map[string]any:
		if ws, ok := workspaceFromMap(x); ok {
			*out = append(*out, ws)
			return
		}
		for k, v := range x {
			if s, ok := v.(string); ok && strings.HasPrefix(s, "/") {
				*out = append(*out, Workspace{Name: k, Path: s})
				continue
			}
			collect(v, out)
		}
	}
}

func workspaceFromMap(m map[string]any) (Workspace, bool) {
	name := firstString(m, "name", "workspace", "id")
	path := firstString(m, "path", "root", "repo", "repo_path", "worktree")
	if name == "" || path == "" {
		return Workspace{}, false
	}
	return Workspace{Name: name, Path: path}, true
}

func firstString(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, ok := m[key]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
	}
	return ""
}
