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

import (
	"fmt"
)

var (
	scrapers map[string]Scraper = make(map[string]Scraper)
)

// AllScrapers returns a list of all registered scrapers.
func AllScrapers() []Scraper {
	all := make([]Scraper, 0)
	for _, s := range scrapers {
		all = append(all, s)
	}

	return all
}

// EnabledScrapers returns a list of all enabled scrapers.
func EnabledScrapers() []Scraper {
	enabled := make([]Scraper, 0)
	for _, s := range scrapers {
		if s.Enabled() {
			enabled = append(enabled, s)
		}
	}

	return enabled
}

func IsScraperEnabled(name string) bool {
	s, ok := scrapers[name]
	if !ok {
		return false
	}
	return s.Enabled()
}

func LookupScraper(name string) (Scraper, bool) {
	s, ok := scrapers[name]
	if !ok {
		return nil, false
	}
	return s, true
}

func SetScraperEnabled(name string, enabled bool) {
	s, ok := scrapers[name]
	if ok {
		s.SetEnabled(enabled)
	}
}

func registerScraper(s Scraper, args ...*argDef) {
	makeFlagsForScraper(
		s,
		args,
		func(enabled bool, args []Arg) {
			s.SetEnabled(enabled)
			if cfg, ok := s.(Configurable); ok {
				if err := cfg.Configure(args...); err != nil {
					if err != nil {
						panic(fmt.Sprintf("bug: failed to make scraper with name %s: %v", s.Name(), err))
					}
				}
			}
			if _, ok := scrapers[s.Name()]; ok {
				panic(fmt.Sprintf("bug: scraper with name %s is already registered", s.Name()))
			}
			scrapers[s.Name()] = s
		},
	)
}
