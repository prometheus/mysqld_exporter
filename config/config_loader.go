// Copyright 2018 The Prometheus Authors
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

type ConfigLoader interface {
	Load() (*Config, error)
}

type defaultConfigLoader struct {
	baseConfig *Config
	configPath string
}

// DefaultConfigLoader returns a ConfigLoader that produces *Config by
// merging the config located at configPath with a *Config produced from
// program state.
func DefaultConfigLoader(configPath string) ConfigLoader {
	return &defaultConfigLoader{
		baseConfig: FromState(),
		configPath: configPath,
	}
}

// Load produces a *Config in the following way:
//
// 1. Obtains and stores an immutable base *Config from program state.
// 2. On each call to Load(), loads a *Config from configPath
func (dcf *defaultConfigLoader) Load() (*Config, error) {
	// Clone base config.
	newConfig := dcf.baseConfig.Clone()

	// If a config file is empty, return new config.
	if dcf.configPath == "" {
		return newConfig, nil
	}

	// Otherwise, load and merge file config into new config.
	fileConfig, err := FromFile(dcf.configPath)
	if err != nil {
		return nil, err
	}
	newConfig.Merge(fileConfig)

	return newConfig, nil
}
