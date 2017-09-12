// Copyright 2017 Percona LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package helpers provides test helpers for Prometheus exporters.
//
// It contains workarounds for the following issues:
//  * https://github.com/prometheus/client_golang/issues/322
//  * https://github.com/prometheus/client_golang/issues/323
package helpers

import (
	"fmt"
	"regexp"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

var nameAndHelpRE = regexp.MustCompile(`fqName: "(\w+)", help: "([^"]+)"`)

func getNameAndHelp(d *prometheus.Desc) (string, string) {
	m := nameAndHelpRE.FindStringSubmatch(d.String())
	if len(m) != 3 {
		panic(fmt.Sprintf("failed to get metric name and help from %#q: %#v", d.String(), m))
	}
	return m[1], m[2]
}

// Metric contains Prometheus metric details.
type Metric struct {
	Name   string
	Help   string
	Labels prometheus.Labels
	Type   dto.MetricType
	Value  float64
}

// ReadMetric extracts details from Prometheus metric.
func ReadMetric(m prometheus.Metric) *Metric {
	pb := &dto.Metric{}
	if err := m.Write(pb); err != nil {
		panic(err)
	}

	name, help := getNameAndHelp(m.Desc())
	labels := make(prometheus.Labels, len(pb.Label))
	for _, v := range pb.Label {
		labels[v.GetName()] = v.GetValue()
	}
	if pb.Gauge != nil {
		return &Metric{name, help, labels, dto.MetricType_GAUGE, pb.GetGauge().GetValue()}
	}
	if pb.Counter != nil {
		return &Metric{name, help, labels, dto.MetricType_COUNTER, pb.GetCounter().GetValue()}
	}
	if pb.Untyped != nil {
		return &Metric{name, help, labels, dto.MetricType_UNTYPED, pb.GetUntyped().GetValue()}
	}
	panic("Unsupported metric type")
}
