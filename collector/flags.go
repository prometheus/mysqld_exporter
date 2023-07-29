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
	"strconv"

	"github.com/alecthomas/kingpin/v2"
)

func makeFlagsForScraper(s Scraper, argDefs []*argDef, result func(bool, []Arg)) {
	// Register collector enabled flag.
	name := "collect." + s.Name()
	help := s.Help()
	if s.Enabled() {
		help = fmt.Sprintf("%s (Enabled by default)", help)
	}
	enabled := kingpin.Flag(
		name,
		help,
	).Default(strconv.FormatBool(s.Enabled())).Bool()

	var args []Arg
	for _, loopArgDef := range argDefs {
		argDef := loopArgDef
		name := name + "." + argDef.name

		help := argDef.help
		af := kingpin.Flag(name, help)

		switch argDef.defaultValue.(type) {
		case bool:
			enabled := argDef.defaultValue.(bool)
			if enabled {
				af.Help(fmt.Sprintf("%s (Enabled by default)", help))
			}
			d := strconv.FormatBool(enabled)
			value := af.Default(d).Bool()
			kingpin.CommandLine.Action(func(*kingpin.ParseContext) error {
				args = append(args, NewArg(argDef.name, *value))
				return nil
			})
		case int:
			d := argDef.defaultValue.(int)
			value := af.Default(strconv.FormatInt(int64(d), 10)).Int()
			kingpin.CommandLine.Action(func(*kingpin.ParseContext) error {
				args = append(args, NewArg(argDef.name, *value))
				return nil
			})
		case string:
			d := argDef.defaultValue.(string)
			value := af.Default(d).String()
			kingpin.CommandLine.Action(func(*kingpin.ParseContext) error {
				args = append(args, NewArg(argDef.name, *value))
				return nil
			})
		}
	}

	kingpin.CommandLine.Action(func(*kingpin.ParseContext) error {
		result(*enabled, args)
		return nil
	})
}
