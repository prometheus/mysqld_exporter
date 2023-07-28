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

// DefaultLoader returns a *Config generating function that produces configs in
// this way:
//
// 1. Takes the config as it exists at program start.
// 2. Merges in config from CLI flags.
// 3. Merges in config from config file (if non-empty).
func DefaultLoader(configPath string) func() (*Config, error) {
	// Clone config as it exists at program start.
	baseConfig := FromRegistry().Clone()

	return func() (*Config, error) {
		// Clone base config.
		newConfig := baseConfig.Clone()

		// Merge in config from flags.
		fromFlags, err := FromFlags()
		if err != nil {
			return nil, err
		}
		baseConfig.Merge(fromFlags)

		// If a config file is empty, return current config.
		if configPath == "" {
			return newConfig, nil
		}

		// Otherwise, load and merge config.
		fileConfig, err := FromFile(configPath)
		if err != nil {
			return nil, err
		}

		newConfig.Merge(fileConfig)

		return newConfig, nil
	}
}
