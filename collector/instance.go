// Copyright 2024 The Prometheus Authors
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
	"database/sql"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/blang/semver/v4"
)

const (
	FlavorMySQL   = "mysql"
	FlavorMariaDB = "mariadb"
)

type instance struct {
	db                *sql.DB
	flavor            string
	version           semver.Version
	versionMajorMinor float64
}

func newInstance(dsn string) (*instance, error) {
	i := &instance{}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	i.db = db

	version, versionString, err := queryVersion(db)
	if err != nil {
		db.Close()
		return nil, err
	}

	i.version = version

	versionMajorMinor, err := strconv.ParseFloat(fmt.Sprintf("%d.%d", i.version.Major, i.version.Minor), 64)
	if err != nil {
		db.Close()
		return nil, err
	}

	i.versionMajorMinor = versionMajorMinor

	if strings.Contains(strings.ToLower(versionString), "mariadb") {
		i.flavor = FlavorMariaDB
	} else {
		i.flavor = FlavorMySQL
	}

	return i, nil
}

func (i *instance) getDB() *sql.DB {
	return i.db
}

func (i *instance) Close() error {
	return i.db.Close()
}

// Ping checks connection availability and possibly invalidates the connection if it fails.
func (i *instance) Ping() error {
	if err := i.db.Ping(); err != nil {
		if cerr := i.Close(); cerr != nil {
			return err
		}
		return err
	}
	return nil
}

// The result of SELECT version() is something like:
// for MariaDB: "10.5.17-MariaDB-1:10.5.17+maria~ubu2004-log"
// for MySQL: "8.0.36-28.1"
var versionRegex = regexp.MustCompile(`^((\d+)(\.\d+)(\.\d+))`)

func queryVersion(db *sql.DB) (semver.Version, string, error) {
	var version string
	err := db.QueryRow("SELECT @@version;").Scan(&version)
	if err != nil {
		return semver.Version{}, version, err
	}

	matches := versionRegex.FindStringSubmatch(version)
	if len(matches) > 1 {
		parsedVersion, err := semver.ParseTolerant(matches[1])
		if err != nil {
			return semver.Version{}, version, fmt.Errorf("could not parse version from %q", matches[1])
		}
		return parsedVersion, version, nil
	}

	return semver.Version{}, version, fmt.Errorf("could not parse version from %q", version)
}
