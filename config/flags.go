// Copyright 2023 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"errors"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/mysqld_exporter/collector"
)

var (
	fromFlags *Config
)

func FromFlags() (*Config, error) {
	if fromFlags == nil {
		return nil, errors.New("cannot use before cli flags have been parsed")
	}
	return fromFlags, nil
}

// makeConfigFromFlags returns a *config.Config populated by user-provided CLI flags.
// The config is not populated untilt he flags are parsed.
func makeFromFlags(flags map[string]*kingpin.FlagClause, setConfigFn func(*Config)) {
	configFromFlags := &Config{}

	// Process scrapers.
	for _, s := range collector.AllScrapers() {
		// Get scraper enablement flag.
		cf, ok := flags["collect."+s.Name()]
		if !ok {
			continue
		}

		// Was it enabled by the user?
		enabledByUser := false
		cf.IsSetByUser(&enabledByUser)

		// If so, add c to config.
		c := &Collector{}
		cf.Action(func(*kingpin.ParseContext) error {
			if !enabledByUser {
				return nil
			}
			c.Name = s.Name()
			c.Enabled = cf.Bool()
			configFromFlags.Collectors = append(configFromFlags.Collectors, c)
			return nil
		})

		// Process scraper args.
		cfg, ok := s.(collector.Configurable)
		if !ok {
			continue
		}

		for _, argDef := range cfg.ArgDefinitions() {
			// Get scraper arg flag.
			af, ok := flags["collect."+s.Name()+"."+argDef.Name()]
			if !ok {
				continue
			}

			// Was it set by the user?
			setByUser := false
			af.IsSetByUser(&setByUser)

			// If so, add arg to collector.
			arg := &Arg{}
			af.Action(func(*kingpin.ParseContext) error {
				if !setByUser {
					return nil
				}
				var value interface{}
				switch argDef.DefaultValue().(type) {
				case bool:
					value = af.Bool()
				case int:
					value = af.Int()
				case string:
					value = af.String()
				}
				arg.Value = value
				c.Args = append(c.Args, arg)
				return nil
			})
		}
	}

	kingpin.CommandLine.Action(func(*kingpin.ParseContext) error {
		setConfigFn(configFromFlags)
		return nil
	})
}

func init() {
	makeFromFlags(collector.AllScraperFlags(), func(config *Config) {
		fromFlags = config
	})
}
