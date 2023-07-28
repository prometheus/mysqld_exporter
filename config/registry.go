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

import "github.com/prometheus/mysqld_exporter/collector"

var (
	fromRegistry *Config
)

func FromRegistry() *Config {
	return fromRegistry
}

func makeFromRegistry() *Config {
	registryConfig := &Config{}

	for _, s := range collector.AllScrapers() {
		enabled := collector.IsScraperEnabled(s.Name())
		c := &Collector{}

		c.Name = s.Name()
		c.Enabled = &enabled

		registryConfig.Collectors = append(registryConfig.Collectors, c)

		cfg, ok := s.(collector.Configurable)
		if !ok {
			continue
		}

		for _, argDef := range cfg.ArgDefinitions() {
			arg := &Arg{}
			arg.Name = argDef.Name()
			arg.Value = argDef.DefaultValue()

			c.Args = append(c.Args, arg)
		}
	}

	return registryConfig
}

func init() {
	fromRegistry = makeFromRegistry()
}
