package datapoint

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvert(t *testing.T) {
	nriaSample := `
[
  {
    "EntityID": 5808559336115300505,
    "IsAgent": true,
    "Events": [
      {
        "eventType": "NetworkSample",
        "timestamp": 1740754070,
        "entityKey": "kind-control-plane",
        "interfaceName": "ip6tnl0",
        "hardwareAddress": "",
        "state": "down",
        "receiveBytesPerSecond": 0,
        "receivePacketsPerSecond": 0
      },
      {
        "eventType": "FooSample",
        "timestamp": 1740754071,
        "entityKey": "kind-control-plane",
        "interfaceName": "eth0",
        "hardwareAddress": "1a:47:5a:e5:50:58",
        "ipV4Address": "10.244.0.178/24",
        "ipV6Address": "fe80::1847:5aff:fee5:5058/64",
        "state": "up",
        "receiveBytesPerSecond": 342.00520312187314,
        "receivePacketsPerSecond": 2.201320792475485
      }
    ]
  }
]`
	dps, err := ReadFrom(strings.NewReader(nriaSample))
	require.NoError(t, err)
	assert.Equal(t, 2, len(dps))
	assert.Equal(t, []DataPointGroup{{
		SampleName: "NetworkSample",
		EntityID:   5808559336115300505,
		MetricAttrs: map[string]string{
			"entityKey":       "kind-control-plane",
			"interfaceName":   "ip6tnl0",
			"hardwareAddress": "",
			"state":           "down",
		},
		DataPoints: []DataPoint{
			{Name: "receiveBytesPerSecond", Value: 0.0, TimestampMS: 1740754070000},
			{Name: "receivePacketsPerSecond", Value: 0.0, TimestampMS: 1740754070000},
		},
	}, {
		SampleName: "FooSample",
		EntityID:   5808559336115300505,
		MetricAttrs: map[string]string{
			"entityKey":       "kind-control-plane",
			"interfaceName":   "eth0",
			"hardwareAddress": "1a:47:5a:e5:50:58",
			"ipV4Address":     "10.244.0.178/24",
			"ipV6Address":     "fe80::1847:5aff:fee5:5058/64",
			"state":           "up",
		},
		DataPoints: []DataPoint{
			{Name: "receiveBytesPerSecond", Value: 342.00520312187314, TimestampMS: 1740754071000},
			{Name: "receivePacketsPerSecond", Value: 2.201320792475485, TimestampMS: 1740754071000},
		},
	}}, dps)
}
