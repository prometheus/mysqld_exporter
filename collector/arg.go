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
	Type() ArgType
}

type arg struct {
	name    string
	argType ArgType
	value   interface{}
}

type boolArgDefinition struct {
	name         string
	help         string
	defaultValue bool
}

type intArgDefinition struct {
	name         string
	help         string
	defaultValue int
}

type stringArgDefinition struct {
	name         string
	help         string
	defaultValue string
}

func (a *arg) Name() string {
	return a.name
}

func (a *arg) Type() ArgType {
	return a.argType
}

func (a *arg) Value() interface{} {
	return a.value
}

func (bad *boolArgDefinition) Name() string {
	return bad.name
}

func (bad *boolArgDefinition) Help() string {
	return bad.help
}

func (bad *boolArgDefinition) Type() ArgType {
	return BoolArgType
}

func (bad *boolArgDefinition) DefaultValue() interface{} {
	return bad.defaultValue
}

func (iad *intArgDefinition) Name() string {
	return iad.name
}

func (iad *intArgDefinition) Help() string {
	return iad.help
}

func (iad *intArgDefinition) Type() ArgType {
	return IntArgType
}

func (iad *intArgDefinition) DefaultValue() interface{} {
	return iad.defaultValue
}

func (sad *stringArgDefinition) Name() string {
	return sad.name
}

func (sad *stringArgDefinition) Help() string {
	return sad.help
}

func (sad *stringArgDefinition) Type() ArgType {
	return BoolArgType
}

func (sad *stringArgDefinition) DefaultValue() interface{} {
	return sad.defaultValue
}

func defaultArgs(argDefs []ArgDefinition) []Arg {
	args := make([]Arg, len(argDefs))
	for i, argDef := range argDefs {
		args[i] = &arg{
			name:    argDef.Name(),
			argType: argDef.Type(),
			value:   argDef.DefaultValue(),
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
