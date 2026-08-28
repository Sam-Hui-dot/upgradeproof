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
	File     string `yaml:"file"`
	Service  string `yaml:"service"`
	ImageEnv string `yaml:"image_env"`
}

type Path struct {
	Name string   `yaml:"name"`
	From string   `yaml:"from"`
	Via  []string `yaml:"via,omitempty"`
	To   Target   `yaml:"to"`
}

type Target struct {
	Image string       `yaml:"image,omitempty"`
	Build *BuildTarget `yaml:"build,omitempty"`
}

type BuildTarget struct {
	Service string `yaml:"service"`
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
	if c.Version != 1 {
		errs = append(errs, fmt.Errorf("version must be 1, got %d", c.Version))
	}
	if strings.TrimSpace(c.Compose.File) == "" {
		errs = append(errs, errors.New("compose.file is required"))
	} else if st, err := os.Stat(filepath.Join(baseDir, c.Compose.File)); err != nil || st.IsDir() {
		errs = append(errs, fmt.Errorf("compose.file does not exist or is not a file: %s", c.Compose.File))
	}
	if strings.TrimSpace(c.Compose.Service) == "" {
		errs = append(errs, errors.New("compose.service is required"))
	}
	if c.Compose.ImageEnv != "UPGRADEPROOF_IMAGE" {
		errs = append(errs, errors.New("compose.image_env must be UPGRADEPROOF_IMAGE"))
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
		if strings.TrimSpace(p.From) == "" {
			errs = append(errs, fmt.Errorf("%s.from is required", prefix))
		}
		for j, image := range p.Via {
			if strings.TrimSpace(image) == "" {
				errs = append(errs, fmt.Errorf("%s.via[%d] must not be empty", prefix, j))
			}
		}
		shapes := 0
		if strings.TrimSpace(p.To.Image) != "" {
			shapes++
		}
		if p.To.Build != nil {
			shapes++
			if strings.TrimSpace(p.To.Build.Service) == "" {
				errs = append(errs, fmt.Errorf("%s.to.build.service is required", prefix))
			} else if p.To.Build.Service != c.Compose.Service {
				errs = append(errs, fmt.Errorf("%s.to.build.service must equal compose.service %q", prefix, c.Compose.Service))
			}
		}
		if shapes != 1 {
			errs = append(errs, fmt.Errorf("%s.to must define exactly one of image or build", prefix))
		}
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
