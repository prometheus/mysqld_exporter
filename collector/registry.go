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

package collector

import "fmt"

var (
	registry map[string]Scraper = make(map[string]Scraper)
)

func All() []Scraper {
	r := make([]Scraper, 0)
	for _, s := range registry {
		r = append(r, s)
	}
	return r
}

func Lookup(name string) (Scraper, error) {
	s, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("scraper with name %s is not registered", name)
	}
	return s, nil
}

func mustRegisterWithDefaults(s Scraper) {
	if err := s.Configure(defaultArgs(s.ArgDefinitions())...); err != nil {
		panic(fmt.Sprintf("bug: %v", err))
	}
	if err := register(s); err != nil {
		panic(fmt.Sprintf("bug: %v", err))
	}
}

func register(s Scraper) error {
	if _, ok := registry[s.Name()]; ok {
		return fmt.Errorf("scraper with name %s is already registered", s.Name())
	}
	registry[s.Name()] = s
	return nil
}
