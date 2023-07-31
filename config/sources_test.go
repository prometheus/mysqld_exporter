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

package config

import (
	"fmt"
	"testing"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/mysqld_exporter/collector"
	"github.com/smartystreets/goconvey/convey"
)

func TestSources(t *testing.T) {
	convey.Convey("FromFile()", t, func() {
		convey.Convey("When the file exists and the contents are valid", func() {
			cfg, err := FromFile("testdata/mysqld_exporter.yml")
			convey.So(err, convey.ShouldBeNil)
			convey.So(cfg, convey.ShouldNotBeNil)

			enabled := true
			notEnabled := false

			expectCfg := Config{
				Collectors: []*Collector{
					{
						Name:    "heartbeat",
						Enabled: &enabled,
						Args: []*Arg{
							{
								Name:  "database",
								Value: "my_heartbeat",
							},
							{
								Name:  "utc",
								Value: true,
							},
						},
					},
					{
						Name: "processlist",
						Args: []*Arg{
							{
								Name:  "min_time",
								Value: 10,
							},
						},
					},
					{
						Name:    "slave_status",
						Enabled: &notEnabled,
					},
				},
			}

			convey.So(*cfg, convey.ShouldEqual, expectCfg)
		})
	})

	convey.Convey("FromState()", t, func() {
		convey.Convey("Before collector registry is initialized", func() {
			cfg := FromState()

			convey.Convey("Config has no collectors", func() {
				convey.So(cfg.Collectors, convey.ShouldBeEmpty)
			})
		})

		convey.Convey("After registry is initialized", func() {
			testApp := kingpin.New(fmt.Sprintf("mysqld_exporter.config.test[%s]", t.Name()), "")
			collector.InitRegistry(testApp)
			testApp.Parse([]string{})
			cfg := FromState()

			convey.Convey("Config has a collector for each scraper", func() {
				convey.So(cfg.Collectors, convey.ShouldNotBeEmpty)
				convey.So(len(cfg.Collectors), convey.ShouldNotEqual, 0)

				sByName := make(map[string]collector.Scraper)
				for _, s := range collector.AllScrapers() {
					sByName[s.Name()] = s
				}

				for _, cfgCol := range cfg.Collectors {
					s, ok := sByName[cfgCol.Name]
					convey.So(ok, convey.ShouldBeTrue)
					convey.So(*cfgCol.Enabled, convey.ShouldEqual, s.Enabled())

					sCfg, ok := s.(collector.Configurable)
					if !ok {
						continue
					}

					convey.So(cfgCol.Args, convey.ShouldHaveLength, len(sCfg.Args()))

					sArgsByName := make(map[string]collector.Arg)
					for _, sArg := range sCfg.Args() {
						sArgsByName[sArg.Name()] = sArg
					}

					for _, colArg := range cfgCol.Args {
						sArg, ok := sArgsByName[colArg.Name]
						convey.So(ok, convey.ShouldBeTrue)
						convey.So(colArg.Value, convey.ShouldEqual, sArg.Value())
					}
				}
			})
		})
	})
}
