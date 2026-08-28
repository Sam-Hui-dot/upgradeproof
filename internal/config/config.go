package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Duration struct{ time.Duration }

func (d *Duration) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.ScalarNode {
		return errors.New("duration must be a string")
	}
	v, err := time.ParseDuration(node.Value)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", node.Value, err)
	}
	if v <= 0 {
		return fmt.Errorf("duration must be greater than zero: %q", node.Value)
	}
	d.Duration = v
	return nil
}

type Config struct {
	Version int           `yaml:"version"`
	Compose ComposeConfig `yaml:"compose"`
	Paths   []Path        `yaml:"paths"`
	Health  HealthConfig  `yaml:"health"`
	Seed    Hook          `yaml:"seed"`
	Verify  VerifyConfig  `yaml:"verify"`
}

type ComposeConfig struct {
	File string `yaml:"file"`
}

type Path struct {
	Name string         `yaml:"name"`
	From ReleaseState   `yaml:"from"`
	Via  []ReleaseState `yaml:"via,omitempty"`
	To   ReleaseState   `yaml:"to"`
}

type ReleaseState struct {
	Env   map[string]string `yaml:"env"`
	Build *BuildTarget      `yaml:"build,omitempty"`
}

type BuildTarget struct {
	Services []string `yaml:"services"`
	TagEnv   string   `yaml:"tag_env"`
}

type HealthConfig struct {
	Type     string   `yaml:"type"`
	URL      string   `yaml:"url"`
	Timeout  Duration `yaml:"timeout"`
	Interval Duration `yaml:"interval"`
}

type Hook struct {
	Command string   `yaml:"command"`
	Timeout Duration `yaml:"timeout"`
}

type VerifyConfig struct {
	Checks []Check `yaml:"checks"`
}

type Check struct {
	Name string `yaml:"name"`
	Hook `yaml:",inline"`
}

func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	var cfg Config
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, errors.New("config must contain exactly one YAML document")
		}
		return nil, fmt.Errorf("decode trailing config: %w", err)
	}
	if err := cfg.Validate(filepath.Dir(path)); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) Validate(baseDir string) error {
	var errs []error
	if c.Version != 2 {
		errs = append(errs, fmt.Errorf("version must be 2, got %d", c.Version))
	}
	if strings.TrimSpace(c.Compose.File) == "" {
		errs = append(errs, errors.New("compose.file is required"))
	} else if st, err := os.Stat(filepath.Join(baseDir, c.Compose.File)); err != nil || st.IsDir() {
		errs = append(errs, fmt.Errorf("compose.file does not exist or is not a file: %s", c.Compose.File))
	}
	if len(c.Paths) == 0 {
		errs = append(errs, errors.New("at least one path is required"))
	}
	seenPaths := map[string]bool{}
	for i, p := range c.Paths {
		prefix := fmt.Sprintf("paths[%d]", i)
		if strings.TrimSpace(p.Name) == "" {
			errs = append(errs, fmt.Errorf("%s.name is required", prefix))
		} else if seenPaths[p.Name] {
			errs = append(errs, fmt.Errorf("duplicate path name %q", p.Name))
		}
		seenPaths[p.Name] = true
		errs = append(errs, validateReleaseState(prefix+".from", p.From, false)...)
		for j, state := range p.Via {
			errs = append(errs, validateReleaseState(fmt.Sprintf("%s.via[%d]", prefix, j), state, false)...)
		}
		errs = append(errs, validateReleaseState(prefix+".to", p.To, true)...)
	}
	if c.Health.Type != "http" {
		errs = append(errs, errors.New("health.type must be http"))
	}
	if u, err := url.ParseRequestURI(c.Health.URL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		errs = append(errs, errors.New("health.url must be an absolute http(s) URL"))
	}
	if c.Health.Timeout.Duration <= 0 {
		errs = append(errs, errors.New("health.timeout is required and must be positive"))
	}
	if c.Health.Interval.Duration <= 0 {
		errs = append(errs, errors.New("health.interval is required and must be positive"))
	}
	if strings.TrimSpace(c.Seed.Command) == "" {
		errs = append(errs, errors.New("seed.command is required"))
	}
	if c.Seed.Timeout.Duration <= 0 {
		errs = append(errs, errors.New("seed.timeout is required and must be positive"))
	}
	if len(c.Verify.Checks) == 0 {
		errs = append(errs, errors.New("at least one verify.checks entry is required"))
	}
	seenChecks := map[string]bool{}
	for i, check := range c.Verify.Checks {
		if strings.TrimSpace(check.Name) == "" {
			errs = append(errs, fmt.Errorf("verify.checks[%d].name is required", i))
		} else if seenChecks[check.Name] {
			errs = append(errs, fmt.Errorf("duplicate check name %q", check.Name))
		}
		seenChecks[check.Name] = true
		if strings.TrimSpace(check.Command) == "" {
			errs = append(errs, fmt.Errorf("verify.checks[%d].command is required", i))
		}
		if check.Timeout.Duration <= 0 {
			errs = append(errs, fmt.Errorf("verify.checks[%d].timeout is required and must be positive", i))
		}
	}
	return errors.Join(errs...)
}

func validateReleaseState(prefix string, state ReleaseState, allowBuild bool) []error {
	var errs []error
	if len(state.Env) == 0 {
		errs = append(errs, fmt.Errorf("%s.env must contain at least one variable", prefix))
	}
	for key, value := range state.Env {
		if strings.TrimSpace(key) == "" || strings.Contains(key, "=") {
			errs = append(errs, fmt.Errorf("%s.env contains invalid variable name %q", prefix, key))
		}
		if strings.HasPrefix(strings.ToUpper(key), "UPGRADEPROOF_") {
			errs = append(errs, fmt.Errorf("%s.env variable %q uses reserved UPGRADEPROOF_ prefix", prefix, key))
		}
		if strings.TrimSpace(value) == "" {
			errs = append(errs, fmt.Errorf("%s.env.%s must not be empty", prefix, key))
		}
	}
	if state.Build == nil {
		return errs
	}
	if !allowBuild {
		errs = append(errs, fmt.Errorf("%s.build is only allowed for a target release state", prefix))
	}
	if len(state.Build.Services) == 0 {
		errs = append(errs, fmt.Errorf("%s.build.services must contain at least one service", prefix))
	}
	seen := map[string]bool{}
	for i, service := range state.Build.Services {
		if strings.TrimSpace(service) == "" {
			errs = append(errs, fmt.Errorf("%s.build.services[%d] must not be empty", prefix, i))
		} else if seen[service] {
			errs = append(errs, fmt.Errorf("%s.build.services contains duplicate %q", prefix, service))
		}
		seen[service] = true
	}
	if strings.TrimSpace(state.Build.TagEnv) == "" {
		errs = append(errs, fmt.Errorf("%s.build.tag_env is required", prefix))
	} else if _, ok := state.Env[state.Build.TagEnv]; !ok {
		errs = append(errs, fmt.Errorf("%s.build.tag_env %q must be declared in env", prefix, state.Build.TagEnv))
	}
	return errs
}

func (c *Config) SelectPath(name string) ([]Path, error) {
	if name == "" {
		return c.Paths, nil
	}
	for _, p := range c.Paths {
		if p.Name == name {
			return []Path{p}, nil
		}
	}
	return nil, fmt.Errorf("path %q not found", name)
}
