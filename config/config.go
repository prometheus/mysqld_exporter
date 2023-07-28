// Copyright 2022 The Prometheus Authors
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
	"errors"
	"fmt"
)

type Config struct {
	Collectors []*Collector `yaml:"collect"`
}

type Collector struct {
	Name    string `yaml:"name"`
	Enabled *bool  `yaml:"enabled"`
	Args    []*Arg `yaml:"args"`
}

type Arg struct {
	Name  string      `yaml:"name"`
	Value interface{} `yaml:"value"`
}

// Clone returns a deep copy of c without modifying c.
func (c *Config) Clone() *Config {
	clone := &Config{}
	if c.Collectors == nil {
		return clone
	}
	clone.Collectors = make([]*Collector, len(c.Collectors))
	for i, collector := range c.Collectors {
		clone.Collectors[i] = collector.clone()
	}
	return clone
}

// Merge combines oc with c. Elements in oc take precedence over c. A new
// No modifications are made to oc.
func (c *Config) Merge(oc *Config) {
	if oc == nil {
		return
	}

	if oc.Collectors != nil && c.Collectors == nil {
		c.Collectors = make([]*Collector, 0)
	}

	// Organize c collectors by name.
	cCollectors := make(map[string]*Collector, len(c.Collectors))
	for _, collector := range c.Collectors {
		cCollectors[collector.Name] = collector
	}

	// Range over oc collectors. Update or add to c collectors.
	for _, ocCollector := range oc.Collectors {
		cCollector, ok := cCollectors[ocCollector.Name]
		if !ok {
			c.Collectors = append(c.Collectors, ocCollector.clone())
		} else {
			cCollector.merge(ocCollector)
		}
	}
}

// Validate validates elements of c.
func (c *Config) Validate() error {
	if len(c.Collectors) == 0 {
		return nil
	}
	uniq := make(map[string]bool, len(c.Collectors))
	for _, collector := range c.Collectors {
		if err := collector.validate(); err != nil {
			return fmt.Errorf("collector %s is invalid: %w", collector.Name, err)
		}
		if _, ok := uniq[collector.Name]; ok {
			return fmt.Errorf("duplicate collectors named %s", collector.Name)
		}
		uniq[collector.Name] = true
	}
	return nil
}

// clone returns a deep copy of c without modifying c.
func (c *Collector) clone() *Collector {
	clone := &Collector{}
	clone.Name = c.Name
	if c.Enabled != nil {
		enabled := *c.Enabled
		clone.Enabled = &enabled
	}
	if c.Args == nil {
		return clone
	}
	clone.Args = make([]*Arg, len(c.Args))
	for i, arg := range c.Args {
		clone.Args[i] = arg.clone()
	}
	return clone
}

// merge combines oc with c. Elements in oc take precedence over c. A new
// No modifications are made to oc.
func (c *Collector) merge(oc *Collector) {
	if oc == nil {
		return
	}

	c.Name = oc.Name
	if oc.Enabled != nil {
		enabled := *oc.Enabled
		c.Enabled = &enabled
	}

	if oc.Args != nil && c.Args == nil {
		c.Args = make([]*Arg, 0)
	}

	// Organize c args by name.
	cArgs := make(map[string]*Arg, len(c.Args))
	for _, arg := range c.Args {
		cArgs[arg.Name] = arg
	}

	// Range over oc args, updating or adding to c args.
	for _, ocArg := range oc.Args {
		cArg, ok := cArgs[ocArg.Name]
		if !ok {
			c.Args = append(c.Args, ocArg.clone())
		} else {
			cArg.merge(ocArg)
		}
	}
}

// validate validates elements of c.
func (c *Collector) validate() error {
	if c.Name == "" {
		return errors.New("name must not be empty")
	}
	if len(c.Args) == 0 {
		return nil
	}
	uniq := make(map[string]bool, len(c.Args))
	for _, arg := range c.Args {
		if _, ok := uniq[arg.Name]; ok {
			return fmt.Errorf("duplicate args named %s", arg.Name)
		}
		if err := arg.validate(); err != nil {
			return fmt.Errorf("arg %s is invalid: %w", arg.Name, err)
		}
		uniq[arg.Name] = true
	}
	return nil
}

// clone returns a deep copy of c without modifying c.
func (c *Arg) clone() *Arg {
	clone := &Arg{}
	clone.Name = c.Name
	clone.Value = c.Value
	return clone
}

// merge combines oc with c. Elements in oc take precedence over c. A new
// No modifications are made to oc.
func (c *Arg) merge(oc *Arg) {
	if oc == nil {
		return
	}

	c.Name = oc.Name
	if oc.Value != nil {
		c.Value = oc.Value
	}
}

// validate validates elements of c.
func (c *Arg) validate() error {
	if c.Name == "" {
		return errors.New("name must not be empty")
	}
	if c.Value == nil {
		return nil
	}
	switch c.Value.(type) {
	case bool:
		return nil
	case int:
		return nil
	case string:
		return nil
	}
	return errors.New("invalid type, must be [bool, int, string]")
}
