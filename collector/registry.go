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
	scrapersByName   map[string]Scraper = make(map[string]Scraper)
	enabledByScraper map[Scraper]bool   = make(map[Scraper]bool)
)

func All() map[Scraper]bool {
	cp := make(map[Scraper]bool)
	for s, e := range enabledByScraper {
		cp[s] = e
	}
	return cp
}

func Lookup(name string) (Scraper, bool, error) {
	s, ok := scrapersByName[name]
	if !ok {
		return nil, false, fmt.Errorf("scraper with name %s is not registered", name)
	}
	return s, enabledByScraper[s], nil
}

func mustRegisterWithDefaults(s Scraper, enabled bool) {
	if err := s.Configure(defaultArgs(s.ArgDefinitions())...); err != nil {
		panic(fmt.Sprintf("bug: %v", err))
	}
	if err := register(s, enabled); err != nil {
		panic(fmt.Sprintf("bug: %v", err))
	}
}

func register(s Scraper, enabled bool) error {
	if _, ok := scrapersByName[s.Name()]; ok {
		return fmt.Errorf("scraper with name %s is already registered", s.Name())
	}
	scrapersByName[s.Name()] = s
	enabledByScraper[s] = enabled
	return nil
}
