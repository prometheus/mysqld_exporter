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
	"reflect"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

type labelMap map[string]string
type MetricResult struct {
	name       string
	help       string
	labels     labelMap
	value      float64
	metricType dto.MetricType
}

func readMetric(m prometheus.Metric) MetricResult {

	a := m.Desc()
	pb := &dto.Metric{}
	m.Write(pb)

	name := reflect.ValueOf(a).Elem().FieldByName("fqName").String()
	help := reflect.ValueOf(a).Elem().FieldByName("help").String()
	labels := labelMap{}

	for _, v := range pb.Label {
		labels[v.GetName()] = v.GetValue()
	}

	if pb.Gauge != nil {
		return MetricResult{name: name, help: help, labels: labels, value: pb.GetGauge().GetValue(), metricType: dto.MetricType_GAUGE}
	}
	if pb.Counter != nil {
		return MetricResult{name: name, help: help, labels: labels, value: pb.GetCounter().GetValue(), metricType: dto.MetricType_COUNTER}
	}
	if pb.Untyped != nil {
		return MetricResult{name: name, help: help, labels: labels, value: pb.GetUntyped().GetValue(), metricType: dto.MetricType_UNTYPED}
	}
	panic("Unsupported metric type")
}

func sanitizeQuery(q string) string {
	q = strings.Join(strings.Fields(q), " ")
	q = strings.Replace(q, "(", "\\(", -1)
	q = strings.Replace(q, ")", "\\)", -1)
	q = strings.Replace(q, "*", "\\*", -1)
	return q
}
