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
	"fmt"
	"testing"

	"gopkg.in/ini.v1"

	"github.com/smartystreets/goconvey/convey"
)

func TestValidateConfig(t *testing.T) {
	const (
		client = `
            [client]
            user = root
            password = abc
            [client.server1]
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
		cfg     *ini.File
		section *ini.Section
		opts    = ini.LoadOptions{
			// MySQL ini file can have boolean keys.
			AllowBooleanKeys: true,
		}
		err error
	)
	convey.Convey("Various multi-host exporter config validation", t, func() {
		convey.Convey("No fakeclient section", func() {
			if cfg, err = ini.LoadSources(opts, []byte(client)); err != nil {
				t.Error(err)
			}
			_, err := validateMyConfig(cfg, "fakeclient")
			convey.So(err, convey.ShouldResemble, fmt.Errorf("configuration section not found - [fakeclient]"))
		})
		convey.Convey("Client without user", func() {
			if cfg, err = ini.LoadSources(opts, []byte(missingUser)); err != nil {
				t.Error(err)
			}
			_, err := validateMyConfig(cfg, "client")
			convey.So(err, convey.ShouldResemble, fmt.Errorf("no user specified under [client]"))
		})
		convey.Convey("Client without password", func() {
			if cfg, err = ini.LoadSources(opts, []byte(missingPassword)); err != nil {
				t.Error(err)
			}
			_, err := validateMyConfig(cfg, "client")
			convey.So(err, convey.ShouldResemble, fmt.Errorf("no password specified under [client]"))
		})
		convey.Convey("Valid section", func() {
			if cfg, err = ini.LoadSources(opts, []byte(client)); err != nil {
				t.Error(err)
			}
			client, _ := validateMyConfig(cfg, "client")
			convey.So(client, convey.ShouldHaveSameTypeAs, section)
			convey.So(client.Keys(), convey.ShouldHaveLength, 2)
		})
		convey.Convey("Valid child section", func() {
			if cfg, err = ini.LoadSources(opts, []byte(client)); err != nil {
				t.Error(err)
			}
			client, _ := validateMyConfig(cfg, "client.server1")
			convey.So(client, convey.ShouldHaveSameTypeAs, section)
			convey.So(client.Keys(), convey.ShouldHaveLength, 2)
		})
	})
}

func TestFormDSN(t *testing.T) {
	const (
		workingClient = `
            [client]
            user = root
            password = abc
			host = server2:3306
			[client.server1]
            user = root1
            password = abc123
            `
	)
	var (
		cfg     *ini.File
		section *ini.Section
		opts    = ini.LoadOptions{
			// MySQL ini file can have boolean keys.
			AllowBooleanKeys: true,
		}
		err error
	)

	convey.Convey("Host exporter dsn", t, func() {
		convey.Convey("Default Client", func() {
			if cfg, err = ini.LoadSources(opts, []byte(workingClient)); err != nil {
				t.Error(err)
			}
			if section, err = cfg.GetSection("client"); err != nil {
				t.Error(err)
			}
			dsn, _ := formDSN("server2:3306", section)
			convey.So(dsn, convey.ShouldEqual, "root:abc@tcp(server2:3306)/")
		})
		convey.Convey("Host specific Client", func() {
			if cfg, err = ini.LoadSources(opts, []byte(workingClient)); err != nil {
				t.Error(err)
			}
			if section, err = cfg.GetSection("client.server1"); err != nil {
				t.Error(err)
			}
			dsn, _ := formDSN("server1:8000", section)
			convey.So(dsn, convey.ShouldEqual, "root1:abc123@tcp(server1:8000)/")
		})
		convey.Convey("Without explicit port", func() {
			if cfg, err = ini.LoadSources(opts, []byte(workingClient)); err != nil {
				t.Error(err)
			}
			dsn, _ := formDSN("server1", section)
			convey.So(dsn, convey.ShouldEqual, "root1:abc123@tcp(server1:3306)/")
		})
	})
}
