package safety

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var windowsPath = regexp.MustCompile(`^[A-Za-z]:[\\/]`)

func CheckCompose(data []byte, imageService string) error {
	var root map[string]any
	if err := yaml.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("parse compose file: %w", err)
	}
	var problems []error
	services, ok := stringMap(root["services"])
	if !ok || len(services) == 0 {
		return errors.New("compose file has no services")
	}
	selected, ok := stringMap(services[imageService])
	if !ok {
		problems = append(problems, fmt.Errorf("compose service %q does not exist", imageService))
	} else {
		image, _ := selected["image"].(string)
		if !strings.Contains(image, "${UPGRADEPROOF_IMAGE") && !strings.Contains(image, "$UPGRADEPROOF_IMAGE") {
			problems = append(problems, fmt.Errorf("service %q image must interpolate UPGRADEPROOF_IMAGE", imageService))
		}
	}

	for serviceName, raw := range services {
		service, ok := stringMap(raw)
		if !ok {
			continue
		}
		if name, ok := service["container_name"].(string); ok && strings.TrimSpace(name) != "" {
			problems = append(problems, fmt.Errorf("service %q uses fixed container_name", serviceName))
		}
		mounts, _ := service["volumes"].([]any)
		for i, mount := range mounts {
			if err := checkMount(mount); err != nil {
				problems = append(problems, fmt.Errorf("service %q volume[%d]: %w", serviceName, i, err))
			}
		}
	}

	volumes, _ := stringMap(root["volumes"])
	for name, raw := range volumes {
		if raw == nil {
			continue
		}
		volume, ok := stringMap(raw)
		if !ok {
			continue
		}
		if external, ok := volume["external"].(bool); ok && external {
			problems = append(problems, fmt.Errorf("volume %q is external", name))
		}
		if explicit, ok := volume["name"].(string); ok && strings.TrimSpace(explicit) != "" {
			problems = append(problems, fmt.Errorf("volume %q has explicit name %q", name, explicit))
		}
		if opts, ok := stringMap(volume["driver_opts"]); ok {
			typeValue := strings.ToLower(fmt.Sprint(opts["type"]))
			oValue := strings.ToLower(fmt.Sprint(opts["o"]))
			if typeValue == "none" && hasCSVToken(oValue, "bind") {
				problems = append(problems, fmt.Errorf("volume %q uses local-driver bind options", name))
			}
		}
	}
	return errors.Join(problems...)
}

func checkMount(raw any) error {
	switch mount := raw.(type) {
	case string:
		parts := strings.Split(mount, ":")
		if windowsPath.MatchString(mount) && len(parts) > 2 {
			parts = append([]string{parts[0] + ":" + parts[1]}, parts[2:]...)
		}
		if len(parts) < 2 {
			return nil
		}
		source := parts[0]
		readOnly := len(parts) >= 3 && hasCSVToken(parts[len(parts)-1], "ro")
		if isBindSource(source) && !readOnly {
			return fmt.Errorf("writable bind mount source %q", source)
		}
	case map[string]any:
		source := fmt.Sprint(mount["source"])
		typeValue := strings.ToLower(fmt.Sprint(mount["type"]))
		readOnly, _ := mount["read_only"].(bool)
		if (typeValue == "bind" || (typeValue == "" && isBindSource(source))) && !readOnly {
			return fmt.Errorf("writable bind mount source %q", source)
		}
	default:
		if converted, ok := stringMap(raw); ok {
			return checkMount(converted)
		}
	}
	return nil
}

func isBindSource(source string) bool {
	source = strings.TrimSpace(source)
	return strings.HasPrefix(source, "/") || strings.HasPrefix(source, "./") ||
		strings.HasPrefix(source, "../") || strings.HasPrefix(source, "~") ||
		strings.HasPrefix(source, "\\") || windowsPath.MatchString(source) ||
		strings.Contains(source, "${") || strings.HasPrefix(source, "$")
}

func hasCSVToken(value, want string) bool {
	for _, item := range strings.Split(value, ",") {
		if strings.EqualFold(strings.TrimSpace(item), want) {
			return true
		}
	}
	return false
}

func stringMap(v any) (map[string]any, bool) {
	if v == nil {
		return nil, false
	}
	if m, ok := v.(map[string]any); ok {
		return m, true
	}
	return nil, false
}
