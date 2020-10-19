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

package main

import (
	"gopkg.in/ini.v1"
	"testing"

	"github.com/smartystreets/goconvey/convey"
)

func TestValidateMultiHostExporterConfig(t *testing.T) {
	const (
		missingClient = `
            [Foo]
            user = root
            password = abc
			`
		missingUser = `
            [client]
            password = abc
            `
		missingPassword = `
            [client]
            user = abc
            `
	)

	var (
		cfg  *ini.File
		opts = ini.LoadOptions{
			// MySQL ini file can have boolean keys.
			AllowBooleanKeys: true,
		}
		err error
	)
	convey.Convey("Various multi-host exporter config validation", t, func() {
		convey.Convey("No parent client", func() {
			if cfg, err = ini.LoadSources(opts, []byte(missingClient)); err != nil {
				t.Error(err)
			}
			err := validateMultiHostExporterConfig(cfg)
			convey.So(err, convey.ShouldResemble, errclientParentIsNotSet)
		})
		convey.Convey("Client without user", func() {
			if cfg, err = ini.LoadSources(opts, []byte(missingUser)); err != nil {
				t.Error(err)
			}
			err := validateMultiHostExporterConfig(cfg)
			convey.So(err, convey.ShouldResemble, errUserIsNotSet)
		})
		convey.Convey("Client without password", func() {
			if cfg, err = ini.LoadSources(opts, []byte(missingPassword)); err != nil {
				t.Error(err)
			}
			err := validateMultiHostExporterConfig(cfg)
			convey.So(err, convey.ShouldResemble, errPasswordIsNotSet)
		})
	})
}

func TestFormMultiHostExporterDSN(t *testing.T) {
	const (
		workingClient = `
            [client]
            user = root
            password = abc
			[client.server1]
            user = root1
            password = abc123
            `
	)
	var (
		cfg  *ini.File
		opts = ini.LoadOptions{
			// MySQL ini file can have boolean keys.
			AllowBooleanKeys: true,
		}
		err error
	)

	convey.Convey("Multi Host exporter dsn", t, func() {
		convey.Convey("Default Client", func() {
			if cfg, err = ini.LoadSources(opts, []byte(workingClient)); err != nil {
				t.Error(err)
			}
			dsn, _ := formMultiHostExporterDSN("server2:3306", cfg)
			convey.So(dsn, convey.ShouldEqual, "root:abc@tcp(server2:3306)/")
		})
		convey.Convey("Host specific Client", func() {
			if cfg, err = ini.LoadSources(opts, []byte(workingClient)); err != nil {
				t.Error(err)
			}
			dsn, _ := formMultiHostExporterDSN("server1:8000", cfg)
			convey.So(dsn, convey.ShouldEqual, "root1:abc123@tcp(server1:8000)/")
		})
		convey.Convey("Without explicit port", func() {
			if cfg, err = ini.LoadSources(opts, []byte(workingClient)); err != nil {
				t.Error(err)
			}
			dsn, _ := formMultiHostExporterDSN("server1", cfg)
			convey.So(dsn, convey.ShouldEqual, "root1:abc123@tcp(server1:3306)/")
		})

	})
}
