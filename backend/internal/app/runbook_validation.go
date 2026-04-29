package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var runbookSlugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func validateRunbookSlug(raw string) (string, error) {
	slug := strings.TrimSpace(strings.ToLower(raw))
	if slug == "" {
		return "", errors.New("slug is required")
	}
	if len(slug) > 120 {
		return "", errors.New("slug is too long")
	}
	if !runbookSlugPattern.MatchString(slug) {
		return "", errors.New("invalid slug format")
	}
	return slug, nil
}

func normalizeTags(tags []string) string {
	if len(tags) == 0 {
		return ""
	}
	seen := make(map[string]struct{}, len(tags))
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		t := strings.TrimSpace(strings.ToLower(tag))
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	return strings.Join(out, ",")
}

func splitTags(tags string) []string {
	raw := strings.Split(strings.TrimSpace(tags), ",")
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func validateRunbookTargetType(raw string) (string, error) {
	target := strings.TrimSpace(strings.ToLower(raw))
	switch target {
	case "server", "service", "cluster":
		return target, nil
	default:
		return "", errors.New("invalid target_type")
	}
}

func validateRunbookDefinition(raw json.RawMessage) (map[string]any, error) {
	if len(raw) == 0 {
		return nil, errors.New("definition is required")
	}
	var definition map[string]any
	if err := json.Unmarshal(raw, &definition); err != nil {
		return nil, errors.New("invalid definition json")
	}
	stepsRaw, ok := definition["steps"]
	if !ok {
		return nil, errors.New("steps are required")
	}
	steps, ok := stepsRaw.([]any)
	if !ok || len(steps) == 0 {
		return nil, errors.New("steps must be a non-empty array")
	}
	for idx, stepRaw := range steps {
		stepMap, ok := stepRaw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("step %d must be object", idx)
		}
		stepType := strings.TrimSpace(strings.ToLower(fmt.Sprintf("%v", stepMap["type"])))
		if stepType == "" {
			return nil, fmt.Errorf("step %d type is required", idx)
		}
		switch stepType {
		case "shell", "script", "condition":
		default:
			return nil, fmt.Errorf("step %d has invalid type", idx)
		}
		if stepType == "shell" || stepType == "script" {
			command := strings.TrimSpace(fmt.Sprintf("%v", stepMap["command"]))
			if command == "" {
				return nil, fmt.Errorf("step %d command is required", idx)
			}
		}
	}
	return definition, nil
}
