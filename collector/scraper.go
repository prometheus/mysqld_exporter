// Copyright 2018 The Prometheus Authors
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
	"context"
	"database/sql"
	"fmt"

	"github.com/go-kit/log"
	_ "github.com/go-sql-driver/mysql"
	"github.com/prometheus/client_golang/prometheus"
)

// Arg may be accepted by a Configurable Scraper.
type Arg interface {
	// Name of the arg.
	Name() string
	// Value of the arg.
	Value() interface{}
}

// ArgDefinition describes an Arg that a Configurable Scraper will accept.
type ArgDefinition interface {
	// Name of the Arg that this ArgDefinition defines.
	Name() string
	// Help describes the the role of the Arg defined by this ArgDefinition.
	Help() string
	// DefaultValue defines the default value of the Arg defined by this
	// ArgDefinition.
	DefaultValue() interface{}
}

// Scraper is minimal interface that let's you add new prometheus metrics to mysqld_exporter.
type Scraper interface {
	// Name of the Scraper. Should be unique.
	Name() string

	// Help describes the role of the Scraper.
	// Example: "Collect from SHOW ENGINE INNODB STATUS"
	Help() string

	// Version of MySQL from which scraper is available.
	Version() float64

	// Scrape collects data from database connection and sends it over channel as prometheus metric.
	Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric, logger log.Logger) error
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

// NewArg creates an Arg from with the provided name and value.
func NewArg(name string, value interface{}) Arg {
	return &arg{name, value}
}

// Name of the arg.
func (a *arg) Name() string {
	return a.name
}

// Value of the arg.
func (a *arg) Value() interface{} {
	return a.value
}

// Name of the Arg that this ArgDefinition defines.
func (a *argDefinition) Name() string {
	return a.name
}

// Help describes the the role of the Arg defined by this ArgDefinition.
func (a *argDefinition) Help() string {
	return a.help
}

// DefaultValue defines the default value of the Arg defined by this
// ArgDefinition.
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

// Configurable is an optional interface that Scrapers can implement to
// advertise and accept configuration.
type Configurable interface {
	// ArgDefinitions describes the names, types, and default values of any
	// arguments accepted by the Scraper.
	ArgDefinitions() []ArgDefinition

	// Configure the Scraper.
	Configure(...Arg) error
}
