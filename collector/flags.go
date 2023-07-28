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

func makeFlagsFromScraper(s Scraper, enabled bool) map[string]*kingpin.FlagClause {
	flags := make(map[string]*kingpin.FlagClause)

	// Register collector enabled flag.
	name := "collect." + s.Name()
	help := s.Help()
	if enabled {
		help = fmt.Sprintf("%s (Enabled by default)", help)
	}
	ef := kingpin.Flag(name, help)
	ef.Default(strconv.FormatBool(enabled)).Bool()
	flags[name] = ef

	// Register collector args flags.
	cfg, ok := s.(Configurable)
	if !ok {
		return flags
	}
	for _, argDef := range cfg.ArgDefinitions() {
		name := s.Name() + "." + argDef.Name()

		help := argDef.Help()
		af := kingpin.Flag(name, help)

		switch argDef.DefaultValue().(type) {
		case bool:
			enabled := argDef.DefaultValue().(bool)
			af.Default(strconv.FormatBool(enabled)).Bool()
			if enabled {
				af.Help(fmt.Sprintf("%s (Enabled by default)", help))
			}
		case int:
			i := argDef.DefaultValue().(int)
			af.Default(strconv.FormatInt(int64(i), 10)).Int()
		case string:
			af.Default(argDef.DefaultValue().(string)).String()
		}

		flags[name] = af
	}

	return flags
}
