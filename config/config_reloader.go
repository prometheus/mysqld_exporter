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
	"sync"
)

type ConfigReloader interface {
	Config() *Config
	Reload() error
}

type ConfigReloaderOpts struct {
	Load func() (*Config, error)
}

type configReloader struct {
	sync.RWMutex

	config *Config
	loader func() (*Config, error)
}

func NewConfigReloader(loader func() (*Config, error)) ConfigReloader {
	return &configReloader{loader: loader}
}

func (r *configReloader) Config() *Config {
	r.RLock()
	defer r.RUnlock()
	return r.config
}

func (r *configReloader) Reload() (err error) {
	r.Lock()
	defer r.Unlock()

	config, err := r.loader()
	if err != nil {
		return err
	}
	r.config = config

	return nil
}
