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

func makeFlagsForScraper(
	app *kingpin.Application,
	s Scraper,
	argDefs []*argDef,
	onCommandLineParsed func(enabled bool, args []Arg),
) {
	// Register collector enabled flag.
	name := "collect." + s.Name()
	help := s.Help()
	if s.Enabled() {
		help = fmt.Sprintf("%s (Enabled by default)", help)
	}
	enabled := app.Flag(
		name,
		help,
	).Default(strconv.FormatBool(s.Enabled())).Bool()

	var makeArgs [](func() Arg)
	for _, loopArgDef := range argDefs {
		argDef := loopArgDef
		name := name + "." + argDef.name

		help := argDef.help
		af := app.Flag(name, help)

		switch argDef.defaultValue.(type) {
		case bool:
			enabled := argDef.defaultValue.(bool)
			if enabled {
				af.Help(fmt.Sprintf("%s (Enabled by default)", help))
			}
			d := strconv.FormatBool(enabled)
			value := af.Default(d).Bool()
			makeArgs = append(makeArgs, func() Arg {
				return NewArg(argDef.name, *value)
			})
		case int:
			d := argDef.defaultValue.(int)
			value := af.Default(strconv.FormatInt(int64(d), 10)).Int()
			makeArgs = append(makeArgs, func() Arg {
				return NewArg(argDef.name, *value)
			})
		case string:
			d := argDef.defaultValue.(string)
			value := af.Default(d).String()
			makeArgs = append(makeArgs, func() Arg {
				return NewArg(argDef.name, *value)
			})
		}
	}

	app.Action(func(*kingpin.ParseContext) error {
		var args []Arg
		for _, makeArg := range makeArgs {
			args = append(args, makeArg())
		}
		onCommandLineParsed(*enabled, args)
		return nil
	})
}
