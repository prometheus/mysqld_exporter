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

type ArgType int

const (
	BoolArgType ArgType = iota
	IntArgType
	StringArgType
)

type Arg interface {
	Name() string
	Value() interface{}
}

type ArgDefinition interface {
	Name() string
	Help() string
	DefaultValue() interface{}
}

type arg struct {
	name  string
	value interface{}
}

type argDefinition struct {
	name         string
	help         string
	defaultValue interface{}
}

func (a *arg) Name() string {
	return a.name
}

func (a *arg) Value() interface{} {
	return a.value
}

func (a *argDefinition) Name() string {
	return a.name
}

func (a *argDefinition) Help() string {
	return a.help
}

func (a *argDefinition) DefaultValue() interface{} {
	return a.defaultValue
}

func defaultArgs(argDefs []ArgDefinition) []Arg {
	args := make([]Arg, len(argDefs))
	for i, argDef := range argDefs {
		args[i] = &arg{
			name:  argDef.Name(),
			value: argDef.DefaultValue(),
		}
	}
	return args
}

func noArgsAllowedError(scraperName string) error {
	return fmt.Errorf("scraper %s does not accept any args", scraperName)
}

func unknownArgError(scraperName, argName string) error {
	return fmt.Errorf("scraper %s does not accept arg %s", scraperName, argName)
}

func wrongArgTypeError(scraperName, argName string, argValue interface{}) error {
	return fmt.Errorf("scraper %s arg %s value %v has the wrong type", scraperName, argName, argValue)
}
