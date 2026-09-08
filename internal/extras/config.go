// Package extras is the source service for extra connection links: parsing
// external subscriptions, filtering them by rules, targeting by user panel tag.
package extras

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"regexp"
	"time"

	"gopkg.in/yaml.v3"
)

// DefaultUserAgent is the default User-Agent for outgoing requests to external subscriptions.
const DefaultUserAgent = "Shadowrocket/3378 CFNetwork/3860.700.1 Darwin/25.6.0 iPhone15,2"

// DefaultRefresh is the default polling interval for external subscriptions.
const DefaultRefresh = 24 * time.Hour

// MinRefresh is the lower bound of the polling interval (guards against typos like 5s).
const MinRefresh = time.Minute

// Config is the parsed and validated service configuration.
//
// The yaml tags describe the file; the json tags expose the same tree to extras-ui.
// omitempty on the optional fields is there so that YAML generated from the
// struct contains no empty strings; it has no effect on parsing.
type Config struct {
	Defaults      Defaults       `yaml:"defaults,omitempty" json:"defaults"`
	Subscriptions []Subscription `yaml:"subscriptions" json:"subscriptions"`
	StaticEntries []StaticEntry  `yaml:"static_entries,omitempty" json:"static_entries"`
	Users         Users          `yaml:"users" json:"users"`
}

// Defaults holds the default values for all subscriptions.
type Defaults struct {
	UserAgent string `yaml:"user_agent,omitempty" json:"user_agent"`
	Refresh   string `yaml:"refresh,omitempty" json:"refresh"`
}

// Subscription is an external subscription that is parsed on a schedule.
type Subscription struct {
	Name      string `yaml:"name" json:"name"`
	URL       string `yaml:"url" json:"url"`
	UserAgent string `yaml:"user_agent,omitempty" json:"user_agent"`
	Refresh   string `yaml:"refresh,omitempty" json:"refresh"`
	Rules     []Rule `yaml:"rules" json:"rules"`

	// Resolved values, filled in by Validate.
	UA       string        `yaml:"-" json:"-"`
	Interval time.Duration `yaml:"-" json:"-"`
}

// Rule is a filter on the entry name plus the tags given to entries that pass it.
type Rule struct {
	Match string   `yaml:"match" json:"match"`
	Tags  []string `yaml:"tags" json:"tags"`

	Re *regexp.Regexp `yaml:"-" json:"-"`
}

// StaticEntry is a hand-written connection link.
type StaticEntry struct {
	Name string   `yaml:"name" json:"name"`
	URI  string   `yaml:"uri" json:"uri"`
	Tags []string `yaml:"tags" json:"tags"`
}

// Users holds the targeting rules keyed by the user's tag in the panel.
type Users struct {
	DefaultTags []string            `yaml:"default_tags" json:"default_tags"`
	ByPanelTag  map[string][]string `yaml:"by_panel_tag,omitempty" json:"by_panel_tag"`
}

// Load reads the file and parses it via Parse. An empty file is an error: the
// daemon has nothing to do without a config, and silently starting with zero
// subscriptions would mask a broken deploy. extras-ui does not come through
// here: to it an empty file is a template.
func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if isEmptyDocument(raw) {
		return nil, fmt.Errorf("config %s is empty", path)
	}
	return Parse(raw)
}

// isEmptyDocument reports whether yaml.Decoder would parse raw as an empty
// document: a file with no bytes or one consisting solely of comments. Decode
// relies on the same signal (io.EOF) to return an empty Config instead of an
// error; bytes.TrimSpace is no substitute for this check, since comments
// remain non-whitespace bytes after trimming.
func isEmptyDocument(raw []byte) bool {
	var probe Config
	err := yaml.NewDecoder(bytes.NewReader(raw)).Decode(&probe)
	return errors.Is(err, io.EOF)
}

// Parse decodes and validates the config: Decode, then Validate.
func Parse(raw []byte) (*Config, error) {
	cfg, err := Decode(raw)
	if err != nil {
		return nil, err
	}
	if err := Validate(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Decode parses YAML without validation. Unknown fields are an error;
// everything else is accepted as is, which is how extras-ui imports drafts.
// An empty document (empty file or comments only) yields an empty config.
func Decode(raw []byte) (*Config, error) {
	var cfg Config
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		if errors.Is(err, io.EOF) {
			return &cfg, nil
		}
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &cfg, nil
}

// Validate checks the config and fills in the resolved values: UA and Interval
// on subscriptions, compiled Re on rules.
func Validate(cfg *Config) error {
	// Reset the compiled regexps on all rules before any checks: when a modified
	// config is re-validated, no rule may be left holding its old Re, even if an
	// error is found earlier.
	for i := range cfg.Subscriptions {
		for j := range cfg.Subscriptions[i].Rules {
			cfg.Subscriptions[i].Rules[j].Re = nil
		}
	}

	defaultUA := cfg.Defaults.UserAgent
	if defaultUA == "" {
		defaultUA = DefaultUserAgent
	}
	defaultRefresh := DefaultRefresh
	if cfg.Defaults.Refresh != "" {
		d, err := parseRefresh("defaults", cfg.Defaults.Refresh)
		if err != nil {
			return err
		}
		defaultRefresh = d
	}

	seen := map[string]bool{}
	for i := range cfg.Subscriptions {
		s := &cfg.Subscriptions[i]
		if s.Name == "" {
			return fmt.Errorf("subscription #%d: name is required", i+1)
		}
		if seen[s.Name] {
			return fmt.Errorf("subscription %q: duplicate name", s.Name)
		}
		seen[s.Name] = true

		u, err := url.Parse(s.URL)
		if err != nil || u.Scheme != "http" && u.Scheme != "https" || u.Host == "" {
			return fmt.Errorf("subscription %q: url must be an absolute http(s) url, got %q", s.Name, s.URL)
		}

		s.UA = s.UserAgent
		if s.UA == "" {
			s.UA = defaultUA
		}

		s.Interval = defaultRefresh
		if s.Refresh != "" {
			d, err := parseRefresh("subscription "+s.Name, s.Refresh)
			if err != nil {
				return err
			}
			s.Interval = d
		}

		if len(s.Rules) == 0 {
			return fmt.Errorf("subscription %q: at least one rule is required", s.Name)
		}
		for j := range s.Rules {
			r := &s.Rules[j]
			if r.Match == "" {
				return fmt.Errorf("subscription %q rule #%d: match is required", s.Name, j+1)
			}
			if len(r.Tags) == 0 {
				return fmt.Errorf("subscription %q rule #%d: tags are required", s.Name, j+1)
			}
			re, err := regexp.Compile(r.Match)
			if err != nil {
				return fmt.Errorf("subscription %q rule #%d: bad match regexp: %w", s.Name, j+1, err)
			}
			r.Re = re
		}
	}

	for i, e := range cfg.StaticEntries {
		if e.Name == "" || e.URI == "" {
			return fmt.Errorf("static entry #%d: name and uri are required", i+1)
		}
		if len(e.Tags) == 0 {
			return fmt.Errorf("static entry %q: tags are required", e.Name)
		}
	}

	for tag, mixTags := range cfg.Users.ByPanelTag {
		if len(mixTags) == 0 {
			return fmt.Errorf("users.by_panel_tag[%q]: at least one tag is required", tag)
		}
	}

	return nil
}

func parseRefresh(where, raw string) (time.Duration, error) {
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: bad refresh %q: %w", where, raw, err)
	}
	if d < MinRefresh {
		return 0, fmt.Errorf("%s: refresh %q is below minimum %s", where, raw, MinRefresh)
	}
	return d, nil
}
