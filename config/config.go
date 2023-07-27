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
	"fmt"
)

type Config struct {
	Collectors []*Collector `yaml:"collect"`
}

type Collector struct {
	Name    string          `yaml:"name"`
	Enabled *bool           `yaml:"enabled"`
	Args    []*CollectorArg `yaml:"args"`
}

type CollectorArg struct {
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

	// Organize oc collectors by name.
	ocCollectors := make(map[string]*Collector, len(oc.Collectors))
	for _, collector := range oc.Collectors {
		ocCollectors[collector.Name] = collector
	}

	// Range over c collectors, merging in matching oc collectors.
	for _, collector := range c.Collectors {
		ocCollector, ok := ocCollectors[collector.Name]
		if !ok {
			continue
		}
		collector.merge(ocCollector)
	}
}

// Validate validates elements of c.
func (c *Config) Validate() error {
	var uniq map[string]bool
	if len(c.Collectors) > 0 {
		uniq = make(map[string]bool, len(c.Collectors))
	}
	for _, collector := range c.Collectors {
		if _, ok := uniq[collector.Name]; ok {
			return fmt.Errorf("duplicate collectors named %s", collector.Name)
		}
		if err := collector.validate(); err != nil {
			return fmt.Errorf("collector %s is invalid: %w", collector.Name, err)
		}
	}
	return nil
}

// clone returns a deep copy of c without modifying c.
func (c *Collector) clone() *Collector {
	clone := &Collector{}
	clone.Name = c.Name
	clone.Enabled = c.Enabled
	if c.Args == nil {
		return clone
	}
	clone.Args = make([]*CollectorArg, len(c.Args))
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
		*c.Enabled = *oc.Enabled
	}

	// Organize oc args by name.
	ocArgs := make(map[string]*CollectorArg, len(oc.Args))
	for _, arg := range oc.Args {
		ocArgs[arg.Name] = arg
	}

	// Range over c collectors, merging in matching oc collectors.
	for _, arg := range c.Args {
		ocArg, ok := ocArgs[arg.Name]
		if !ok {
			continue
		}
		arg.merge(ocArg)
	}
}

// validate validates elements of c.
func (c *Collector) validate() error {
	var uniq map[string]bool
	if len(c.Args) > 0 {
		uniq = make(map[string]bool, len(c.Args))
	}
	for _, arg := range c.Args {
		if _, ok := uniq[arg.Name]; ok {
			return fmt.Errorf("duplicate args named %s", arg.Name)
		}
		if err := arg.validate(); err != nil {
			return fmt.Errorf("arg %s is invalid: %w", arg.Name, err)
		}
	}
	return nil
}

// clone returns a deep copy of c without modifying c.
func (c *CollectorArg) clone() *CollectorArg {
	clone := &CollectorArg{}
	clone.Name = c.Name
	clone.Value = c.Value
	return clone
}

// merge combines oc with c. Elements in oc take precedence over c. A new
// No modifications are made to oc.
func (c *CollectorArg) merge(oc *CollectorArg) {
	if oc == nil {
		return
	}

	c.Name = oc.Name
	c.Value = oc.Value
}

// validate validates elements of c.
func (c *CollectorArg) validate() error {
	switch typ := c.Value.(type) {
	case bool:
	case int:
	case string:
		return nil
	default:
		fmt.Errorf("invalid type for arg %s: %v", c.Name, typ)
	}
	return nil
}
