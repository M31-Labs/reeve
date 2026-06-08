package hyphaindex

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"gopkg.in/yaml.v3"
	_ "modernc.org/sqlite"
)

type Space struct {
	ObjectID        string         `json:"object_id"`
	SpaceID         string         `json:"space_id"`
	URI             string         `json:"uri"`
	Title           string         `json:"title"`
	Metadata        map[string]any `json:"metadata"`
	MetadataRaw     string         `json:"metadata_raw,omitempty"`
	Fallback        bool           `json:"fallback,omitempty"`
	MetadataWarning string         `json:"metadata_warning,omitempty"`
}

func ReadSpaces(ctx context.Context, dbPath string) ([]Space, error) {
	return ReadSpacesWithFallback(ctx, dbPath, "")
}

func ReadSpacesWithFallback(ctx context.Context, dbPath, hyphaCommand string) ([]Space, error) {
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	if err := conn.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("open hypha index: %w", err)
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT id, space_id, COALESCE(title, ''), COALESCE(metadata_json, '{}')
		FROM objects
		WHERE type = 'space'
		ORDER BY space_id`)
	if err != nil {
		return nil, fmt.Errorf("query space objects: %w", err)
	}
	defer rows.Close()

	var spaces []Space
	for rows.Next() {
		var s Space
		if err := rows.Scan(&s.ObjectID, &s.SpaceID, &s.Title, &s.MetadataRaw); err != nil {
			return nil, err
		}
		metadata, needsFallback, err := decodeMetadata(s.MetadataRaw)
		if err != nil {
			if hyphaCommand == "" {
				return nil, fmt.Errorf("decode metadata for %s: %w", s.ObjectID, err)
			}
			needsFallback = true
		}
		s.Metadata = metadata
		if needsFallback && hyphaCommand != "" {
			fm, err := showFrontmatter(ctx, hyphaCommand, s.ObjectID, "hypha://"+s.SpaceID)
			if err != nil {
				s.MetadataWarning = fmt.Sprintf("fallback frontmatter failed: %v", err)
			} else {
				s.Metadata = fm
				s.Fallback = true
			}
		}
		if uri, ok := StringField(s.Metadata, "uri"); ok {
			s.URI = uri
		} else {
			s.URI = "hypha://" + s.SpaceID
		}
		spaces = append(spaces, s)
	}
	return spaces, rows.Err()
}

func decodeMetadata(raw string) (map[string]any, bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "{}" || raw == "null" {
		return map[string]any{}, true, nil
	}
	out := map[string]any{}
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&out); err != nil {
		return map[string]any{}, true, err
	}
	if len(out) == 0 {
		return out, true, nil
	}
	return out, false, nil
}

func showFrontmatter(ctx context.Context, command, objectID, spaceURI string) (map[string]any, error) {
	qualified := spaceURI + "/object/" + objectID
	args := append(strings.Fields(command), "show", qualified, "--frontmatter")
	if len(args) == 0 {
		return nil, fmt.Errorf("empty hypha command")
	}
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("%s", msg)
	}
	frontmatter := trimFrontmatterFence(string(out))
	var metadata map[string]any
	if err := yaml.Unmarshal([]byte(frontmatter), &metadata); err != nil {
		return nil, err
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	return metadata, nil
}

func trimFrontmatterFence(s string) string {
	lines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		lines = lines[1:]
	}
	if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "---" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}

func StringField(m map[string]any, key string) (string, bool) {
	v, ok := m[key]
	if !ok {
		return "", false
	}
	switch x := v.(type) {
	case string:
		if strings.TrimSpace(x) == "" {
			return "", false
		}
		return x, true
	default:
		return fmt.Sprint(x), true
	}
}

func FloatField(m map[string]any, key string) (float64, bool) {
	v, ok := m[key]
	if !ok {
		return 0, false
	}
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case json.Number:
		f, err := x.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

func NestedStringField(m map[string]any, objectKey, key string) (string, bool) {
	v, ok := m[objectKey]
	if !ok {
		return "", false
	}
	child, ok := v.(map[string]any)
	if !ok {
		return "", false
	}
	return StringField(child, key)
}
