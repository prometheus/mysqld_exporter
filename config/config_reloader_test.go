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
	"errors"
	"testing"

	"github.com/smartystreets/goconvey/convey"
)

type mockLoader struct {
	calls  int
	config *Config
	err    error
}

func (ml *mockLoader) load() (*Config, error) {
	ml.calls++
	if ml.err != nil {
		return nil, ml.err
	}
	return ml.config, nil
}

func TestConfigReloader(t *testing.T) {
	convey.Convey("Reload", t, func() {
		ml := &mockLoader{
			config: &Config{},
		}
		r := NewConfigReloader(ml.load)
		err := r.Reload()

		convey.Convey("Does not return an error", func() {
			convey.So(err, convey.ShouldBeNil)
		})

		convey.Convey("Makes a single call to loader", func() {
			convey.So(ml.calls, convey.ShouldEqual, 1)
		})

		convey.Convey("Stores the new config", func() {
			convey.So(r.Config(), convey.ShouldPointTo, ml.config)
		})

		convey.Convey("When the loader returns an error", func() {
			ml.config = &Config{}
			ml.err = errors.New("something is wrong")

			err := r.Reload()

			convey.Convey("An error is returned", func() {
				convey.So(err, convey.ShouldNotBeNil)
				convey.So(err.Error(), convey.ShouldEqual, "failed to load config: something is wrong")
			})

			convey.Convey("The stored config is not changed", func() {
				convey.So(r.Config(), convey.ShouldNotPointTo, ml.config)
			})
		})

		convey.Convey("When the loader returns an invalid config", func() {
			ml.config = &Config{
				Collectors: []*Collector{
					{}, // unnamed collectors are invalid
				},
			}
			ml.err = nil

			err := r.Reload()

			convey.Convey("An error is returned", func() {
				convey.So(err, convey.ShouldNotBeNil)
				convey.So(err.Error(), convey.ShouldEqual, "failed to validate config: collector  is invalid: name must not be empty")
			})

			convey.Convey("The stored config is not changed", func() {
				convey.So(r.Config(), convey.ShouldNotPointTo, ml.config)
			})
		})
	})
}
