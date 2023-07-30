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
	"fmt"
	"os"

	"gopkg.in/yaml.v2"

	"github.com/prometheus/mysqld_exporter/collector"
)

// FromFile returns a *Config as parsed from a YAML file.
func FromFile(path string) (*Config, error) {
	fileConfig := &Config{}

	var bs []byte
	var err error
	if bs, err = os.ReadFile(path); err != nil {
		return nil, fmt.Errorf("failed to load %s: %w", path, err)
	}

	if err = yaml.Unmarshal(bs, fileConfig); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}

	if err = fileConfig.Validate(); err != nil {
		return nil, fmt.Errorf("config is invalid %s: %w", path, err)
	}

	return fileConfig, err
}

// FromState returns a *Config based on the current program state. Cannot be
// used before the command line is parsed.
func FromState() *Config {
	stateConfig := &Config{}

	for _, s := range collector.AllScrapers() {
		enabled := s.Enabled()
		c := &Collector{}

		c.Name = s.Name()
		c.Enabled = &enabled

		stateConfig.Collectors = append(stateConfig.Collectors, c)

		cfg, ok := s.(collector.Configurable)
		if !ok {
			continue
		}

		for _, sarg := range cfg.Args() {
			arg := &Arg{}
			arg.Name = sarg.Name()
			arg.Value = sarg.Value()

			c.Args = append(c.Args, arg)
		}
	}

	return stateConfig
}
