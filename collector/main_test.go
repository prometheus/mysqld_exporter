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
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/alecthomas/kingpin/v2"
	"github.com/smartystreets/goconvey/convey"
)

func TestMain(m *testing.M) {
	code := m.Run()
	os.Exit(code)
}

func fakeValue(exampleValue interface{}) interface{} {
	switch exampleValue.(type) {
	case bool:
		return !(exampleValue.(bool))
	case int:
		return (exampleValue.(int)) + 1337
	case string:
		return "fake_" + exampleValue.(string)
	}
	panic("bug: cannot produce fake value using example value of unknown type")

}

func valueToString(value interface{}) string {
	switch v := value.(type) {
	case bool:
		return strconv.FormatBool(!v)
	case int:
		return strconv.FormatInt(int64(+1337), 10)
	case string:
		return v
	}
	panic("bug: cannot convert value of unknown type to string")
}

func testScraperCommon(t *testing.T, s Scraper, enabledByDefault bool, argDefs ...*argDef) {
	t.Helper()

	// Create a test kingpin Application.
	testApp := kingpin.New(fmt.Sprintf("mysqld_exporter.collector.test[%s]", t.Name()), "")

	// Initialize global registry with test kingpin Application. CLI flags
	// will be attached to the kingpin application.
	InitRegistry(testApp)

	// Reset the global registry after tests are done.
	defer deinitRegistry()

	// Do an initial CLI parsing. CLI flags can be re-parsed any number of times.
	testApp.Parse([]string{})

	convey.Convey("Test registered scraper", t, func() {
		rs, ok := LookupScraper(s.Name())

		convey.Convey("Is found in global registry", func() {
			convey.So(ok, convey.ShouldBeTrue)
			convey.So(rs.Name(), convey.ShouldEqual, rs.Name())
		})

		msg := "Is enabled by default"
		if !enabledByDefault {
			msg = "Is disabled by default"
		}
		convey.Convey(msg, func() {
			convey.So(rs.Enabled(), convey.ShouldEqual, enabledByDefault)
		})

		convey.Convey("Can be disabled/enabled by command line", func() {
			if enabledByDefault {
				testApp.Parse([]string{"--no-collect." + rs.Name()})
				convey.So(rs.Enabled(), convey.ShouldBeFalse)
				testApp.Parse([]string{"--collect." + rs.Name()})
				convey.So(rs.Enabled(), convey.ShouldBeTrue)
			} else {
				testApp.Parse([]string{"--collect." + rs.Name()})
				convey.So(rs.Enabled(), convey.ShouldBeTrue)
				testApp.Parse([]string{"--no-collect." + rs.Name()})
				convey.So(rs.Enabled(), convey.ShouldBeFalse)
			}
		})

		if len(argDefs) > 0 {
			convey.Convey("Is configurable", func() {
				convey.So(rs, convey.ShouldImplement, (*Configurable)(nil))

				cfg := rs.(Configurable)

				for _, argDef := range argDefs {
					convey.Convey(fmt.Sprintf("With an argument named %s", argDef.name), func() {
						var match Arg
						for _, arg := range cfg.Args() {
							if argDef.name == arg.Name() {
								match = arg
							}
						}

						convey.SoMsg("But does not expose that argument", match, convey.ShouldNotBeNil)

						value := fakeValue(match.Value())

						cliFlag := fmt.Sprintf("--collect.%s.%s", rs.Name(), argDef.name)
						boolValue, valueIsBool := argDef.defaultValue.(bool)
						if valueIsBool && boolValue {
							cliFlag = fmt.Sprintf("--no-collect.%s.%s", rs.Name(), argDef.name)
						}

						convey.Convey(fmt.Sprintf("Via %s flag", cliFlag), func() {
							cliArgs := []string{cliFlag}
							if !valueIsBool {
								cliArgs = append(cliArgs, valueToString(value))
							}
							testApp.Parse(cliArgs)

							for _, arg := range cfg.Args() {
								if match.Name() == arg.Name() {
									convey.So(arg.Value(), convey.ShouldEqual, value)
								}
							}
						})

						convey.Convey("Via Configure()", func() {
							var match Arg
							for _, arg := range cfg.Args() {
								if argDef.name == arg.Name() {
									match = arg
								}
							}

							value := fakeValue(match.Value())
							err := cfg.Configure(NewArg(match.Name(), value))
							convey.So(err, convey.ShouldBeNil)

							for _, arg := range cfg.Args() {
								if match.Name() == arg.Name() {
									convey.So(arg.Value(), convey.ShouldEqual, value)
								}
							}
						})
					})
				}
			})
		}
	})

	convey.Convey("Disable and enabled", t, func() {
		orig := s.Enabled()
		s.SetEnabled(!orig)
		convey.So(s.Enabled(), convey.ShouldEqual, !orig)
		s.SetEnabled(orig)
		convey.So(s.Enabled(), convey.ShouldEqual, orig)
	})
}
