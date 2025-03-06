package datapoint

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/grafana/alloy/internal/component/otelcol/receiver/nria/nriareceiver/internal/nria"
)

// TODO: convert also inventory and events

type nriaSamples []nriaEntitySample

type nriaEntitySample struct {
	// TODO: match entityID with register API to put here the entity name and other RESOURCE-level
	EntityID nria.EntityID
	Events   []map[string]interface{}
}

type DataPointGroup struct {
	SampleName  string
	EntityID    nria.EntityID
	MetricAttrs map[string]string
	DataPoints  []DataPoint
}

type DataPoint struct {
	Name        string
	TimestampSecs int64 // we don't really need timestamp if not for traces
	Value       float64
}

func ReadFrom(nriaJSON io.Reader) ([]DataPointGroup, error) {
	nria := nriaSamples{}
	if err := json.NewDecoder(nriaJSON).Decode(&nria); err != nil {
		return nil, fmt.Errorf("reading NRIA sample: %w", err)
	}
	var results []DataPointGroup
	for _, entity := range nria {
		for _, event := range entity.Events {
			dpg := DataPointGroup{
				MetricAttrs: map[string]string{},
				EntityID:    entity.EntityID,
			}
			convertDataPoint(event, &dpg)
			results = append(results, dpg)
		}
	}
	return results, nil
}

func convertDataPoint(event map[string]interface{}, d *DataPointGroup) {
	var ts int64
	for key, val := range event {
		if key == "eventType" {
			// remove "Sample" from Samples, as the name is confusing
			d.SampleName = strings.TrimSuffix(val.(string),"Sample")
		} else if key == "timestamp" {
			ts = int64(val.(float64))
		} else if strVal, ok := val.(string); ok {
			d.MetricAttrs[key] = strVal
		} else if numVal, ok := val.(float64); ok {
			d.DataPoints = append(d.DataPoints, DataPoint{
				Name:  key,
				Value: numVal,
			})
		} else {
			panic(fmt.Sprintf("unexpected value type %T: %#v", val, val))
		}
	}
	for i := range d.DataPoints {
		d.DataPoints[i].TimestampSecs = ts
	}
}
