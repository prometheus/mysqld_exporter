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

type registerScraperFn func(Scraper, ...*argDef)
type registryInitHook func(registerScraperFn)

type registry struct {
	sync.Mutex

	app       *kingpin.Application
	scrapers  map[string]Scraper
	initHooks []registryInitHook
}

var (
	globalRegistry = newRegistry()
)

// AllScrapers returns a list of all registered scrapers.
func AllScrapers() []Scraper {
	return globalRegistry.allScrapers()
}

// EnabledScrapers returns a list of all enabled scrapers.
func EnabledScrapers() []Scraper {
	return globalRegistry.enabledScrapers()
}

func InitRegistry(app *kingpin.Application) {
	// Lock registry for external use until command line is parsed.
	globalRegistry.Lock()

	globalRegistry.init(app)

	// Only unlock once, so that command line can be re-parsed (for tests).
	var once sync.Once
	app.Action(func(*kingpin.ParseContext) error {
		once.Do(globalRegistry.Unlock)
		return nil
	})
}

// LookupScraper returns the Scraper with the requested name, if found.
func LookupScraper(name string) (Scraper, bool) {
	return globalRegistry.lookupScraper(name)
}

func initRegistry(app *kingpin.Application) {
	globalRegistry.init(app)
}

func newRegistry() *registry {
	return &registry{
		scrapers: make(map[string]Scraper),
	}
}

func onRegistryInit(initHook registryInitHook) {
	globalRegistry.onInit(initHook)
}

func (r *registry) allScrapers() []Scraper {
	r.Lock()
	defer r.Unlock()
	all := make([]Scraper, 0)
	for _, s := range r.scrapers {
		all = append(all, s)
	}

	return all
}

func (r *registry) enabledScrapers() []Scraper {
	enabled := make([]Scraper, 0)
	for _, s := range r.allScrapers() {
		if s.Enabled() {
			enabled = append(enabled, s)
		}
	}

	return enabled
}

func (r *registry) init(app *kingpin.Application) {
	// Use the provided kingpin app.
	r.app = app

	// Reset all scrapers. They will be re-registered by init hooks.
	r.scrapers = make(map[string]Scraper, len(r.initHooks))

	// Register scrapers.
	for _, hook := range r.initHooks {
		hook(r.registerScraper)
	}
}

func (r *registry) lookupScraper(name string) (Scraper, bool) {
	r.Lock()
	defer r.Unlock()
	s, ok := r.scrapers[name]
	if !ok {
		return nil, false
	}
	return s, true
}

func (r *registry) onInit(initHook func(registerScraper registerScraperFn)) {
	r.initHooks = append(r.initHooks, initHook)
}

func (r *registry) registerScraper(s Scraper, argDefs ...*argDef) {
	if _, ok := r.scrapers[s.Name()]; ok {
		panic(fmt.Sprintf("bug: scraper with name %s is already registered", s.Name()))
	}

	r.scrapers[s.Name()] = s

	makeFlagsForScraper(
		r.app,
		s,
		argDefs,
		// This is called once all the scraper's flags are parsed.
		func(enabled bool, args []Arg) {
			s.SetEnabled(enabled)
			if cfg, ok := s.(Configurable); ok {
				if err := cfg.Configure(args...); err != nil {
					if err != nil {
						panic(fmt.Sprintf("bug: failed to make scraper with name %s: %v", s.Name(), err))
					}
				}
			}
		},
	)
}
