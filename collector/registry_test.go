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
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/smartystreets/goconvey/convey"
)

func TestRegistry(t *testing.T) {
	convey.Convey("Global registry", t, func() {
		convey.Convey("AllScrapers()", func() {
			convey.Convey("Initially contains no scrapers", func() {
				convey.So(AllScrapers(), convey.ShouldHaveLength, 0)
			})

			convey.Convey("Until registry is initialized and command line is parsed", func() {
				testApp := kingpin.New(fmt.Sprintf("mysqld_exporter.collector.test[%s]", t.Name()), "")
				InitRegistry(testApp)
				defer deinitRegistry()
				testApp.Parse([]string{})

				convey.So(len(AllScrapers()), convey.ShouldNotEqual, 0)
			})
		})

		convey.Convey("EnabledScrapers()", func() {
			testApp := kingpin.New(fmt.Sprintf("mysqld_exporter.collector.test[%s]", t.Name()), "")
			InitRegistry(testApp)
			defer deinitRegistry()
			testApp.Parse([]string{})

			allScrapers := AllScrapers()
			enabledScrapers := EnabledScrapers()
			enabledScrapersByName := make(map[string]bool)
			for _, s := range enabledScrapers {
				enabledScrapersByName[s.Name()] = true
			}

			// Test is not meaningful unless there are at least two registered scrapers.
			convey.SoMsg("Test is not meaningless: two few scrapers", len(allScrapers), convey.ShouldBeGreaterThan, 1)

			// Test is not meaningful unless there is at least one enabled scraper.
			convey.SoMsg("Test is not meaningless: all scrapers are enabled", len(enabledScrapers), convey.ShouldNotEqual, len(allScrapers))

			convey.Convey("Returns the subset of scrapers that are enabled", func() {
				for _, enabledScraper := range enabledScrapers {
					convey.So(enabledScraper.Enabled(), convey.ShouldBeTrue)
				}

				for _, s := range allScrapers {
					_, ok := enabledScrapersByName[s.Name()]
					convey.So(ok, convey.ShouldEqual, s.Enabled())
				}
			})
		})

		convey.Convey("InitRegistry()", func() {
			testApp := kingpin.New(fmt.Sprintf("mysqld_exporter.collector.test[%s]", t.Name()), "")
			InitRegistry(testApp)
			defer deinitRegistry()

			convey.Convey("Locks registry until command line is parsed", func() {
				scrapersCh := make(chan []Scraper, 1)
				go func() {
					scrapersCh <- AllScrapers()
				}()
				var scrapers []Scraper
				ctx1, _ := context.WithTimeout(context.Background(), 1*time.Second)
				ctx2, _ := context.WithTimeout(context.Background(), 2*time.Second)
				select {
				case scrapers = <-scrapersCh:
				case <-ctx1.Done():
					convey.SoMsg("But it didn't lock registry", scrapers, convey.ShouldBeNil)
				}
				testApp.Parse([]string{})
				select {
				case scrapers = <-scrapersCh:
				case <-ctx2.Done():
					convey.SoMsg("But it didn't unlock registry after", scrapers, convey.ShouldNotBeNil)
				}
			})
		})
	})
}
