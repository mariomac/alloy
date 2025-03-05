// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package nria

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/mariomac/guara/casing"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/mariomac/rotelic/pkg/datapoint"
	"github.com/mariomac/rotelic/pkg/otlp"
)

type EntityID int64

type FakeCollector struct {
	notifiedProxies map[string]interface{}

	exporter *otlp.Exporter
}

func NewService() FakeCollector {
	return FakeCollector{
		notifiedProxies: map[string]interface{}{},
		exporter:        otlp.NewExporter(),
	}
}


func (fc *FakeCollector) NewSample(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writer.WriteHeader(http.StatusNotFound)
		return
	}

	// TODO: handle compression
	datapoints, err := datapoint.ReadFrom(request.Body)
	if err != nil {
		logrus.WithError(err).Error("Reading request body")
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	for _, sample := range datapoints {
		meterProvider, err := fc.exporter.ForEntity(sample.EntityID)
		if err != nil {
			logrus.WithError(err).Error("Creating exporter entity")
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}
		meter := meterProvider.Meter("rotelic")
		prefix := casing.CamelToDots(sample.SampleName)
		var att []attribute.KeyValue
		for k, v := range sample.MetricAttrs {
			att = append(att, attribute.String(k, v))
		}
		metricAttributes := metric.WithAttributes(att...)

		for _, dp := range sample.DataPoints {
			name := prefix + "." + casing.CamelToDots(dp.Name)
			if strings.HasSuffix(dp.Name, "PerSecond") || strings.HasSuffix(dp.Name, "Percent") {
				gauge, err := meter.Float64Gauge(name)
				if err != nil {
					logrus.WithError(err).Error("Creating gauge")
					writer.WriteHeader(http.StatusInternalServerError)
					return
				}
				gauge.Record(request.Context(), dp.Value,
					metricAttributes)
			}
		}
	}
	writer.WriteHeader(http.StatusOK)
}
