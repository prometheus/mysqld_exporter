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
	"sync"

	"github.com/alecthomas/kingpin/v2"
)

type scraperEntry struct {
	enabled bool
	flags   map[string]*kingpin.FlagClause
	scraper Scraper
}

var (
	registryMu      sync.Mutex
	scraperRegistry map[string]*scraperEntry = make(map[string]*scraperEntry)
)

// AllScrapers returns a list of all registered scrapers.
func AllScrapers() []Scraper {
	all := make([]Scraper, 0)
	for _, se := range scraperRegistry {
		all = append(all, se.scraper)
	}

	return all
}

func IsScraperEnabled(name string) bool {
	registryMu.Lock()
	defer registryMu.Unlock()

	se, ok := scraperRegistry[name]
	if !ok {
		return false
	}
	return se.enabled
}

func allFlags() map[string]*kingpin.FlagClause {
	flags := make(map[string]*kingpin.FlagClause)
	for _, s := range scraperRegistry {
		for name, flag := range s.flags {
			flags[name] = flag
		}
	}
	return flags
}

func lookup(name string) (Scraper, bool) {
	se, ok := scraperRegistry[name]
	if !ok {
		return nil, false
	}
	return se.scraper, true
}

func mustRegisterWithDefaults(s Scraper, enabled bool) {
	if cfg, ok := s.(Configurable); ok {
		if err := cfg.Configure(defaultArgs(cfg.ArgDefinitions())...); err != nil {
			panic(fmt.Sprintf("bug: %v", err))
		}
	}
	if err := register(s, enabled); err != nil {
		panic(fmt.Sprintf("bug: %v", err))
	}
}

func register(s Scraper, enabled bool) error {
	if _, ok := scraperRegistry[s.Name()]; ok {
		return fmt.Errorf("scraper with name %s is already registered", s.Name())
	}
	scraperRegistry[s.Name()] = &scraperEntry{
		enabled: enabled,
		flags:   makeFlagsFromScraper(s, enabled),
		scraper: s,
	}
	return nil
}

func setEnabled(name string, enabled bool) {
	registryMu.Lock()
	defer registryMu.Unlock()

	se, ok := scraperRegistry[name]
	if ok {
		se.enabled = enabled
	}
}
