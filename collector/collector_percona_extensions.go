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
	"database/sql"
	"github.com/prometheus/client_golang/prometheus"
)

/* percona private accessors */

const InformationSchema = informationSchema
const Namespace = namespace

func NewDesc(subsystem, name, help string) *prometheus.Desc {
	return newDesc(subsystem, name, help)
}

func ParseStatus(data sql.RawBytes) (float64, bool) {
	return parseStatus(data)
}

func ValidPrometheusName(s string) string {
	return validPrometheusName(s)
}
