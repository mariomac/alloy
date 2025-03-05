// Package otlp provides an otelcol.receiver.otlp component.
package otlp

import (
	"context"
	"fmt"
	"net/http"

	_ "github.com/microsoft/go-mssqldb/azuread" // fixes weird docker build error
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/receiver"
	"github.com/grafana/alloy/internal/featuregate"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.receiver.nri",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

func New(opts component.Options, arguments Arguments) (component.Component, error) {
	return &Component{port: arguments.Port}, nil
}

// Arguments configures the otelcol.receiver.otlp component.
type Arguments struct {
	// Port of the collector API. If omitted, 8080 will be sued
	Port uint32 `alloy:"port,attr,optional"`
	// TODO: handle compression
	// TODO: handle TLS

	// Output configures where to send received data. Required.
	Output *otelcol.ConsumerArguments `alloy:"output,block"`
	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`
}

type Config struct {
	Port uint32
}

var _ receiver.Arguments = Arguments{}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = Arguments{Port: 8080}
	args.DebugMetrics.SetToDefault()
}

// Convert implements receiver.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	return &Config{Port: args.Port}, nil
}

// Extensions implements receiver.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	return make(map[otelcomponent.ID]otelcomponent.Component)
}

// Exporters implements receiver.Arguments.
func (args Arguments) Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// NextConsumers implements receiver.Arguments.
func (args Arguments) NextConsumers() *otelcol.ConsumerArguments {
	return args.Output
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	return nil
}

// DebugMetricsConfig implements receiver.Arguments.
func (args Arguments) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	return args.DebugMetrics
}

type Component struct {
	port uint32
}

func (c *Component) Update(args component.Arguments) error {
	c.port = args.(Arguments).Port
	return nil
}

func (c *Component) CurrentHealth() component.Health {
	//TODO implement me
	panic("implement me")
}

var _ component.HealthComponent = (*Component)(nil)

func (c *Component) Run(ctx context.Context) error {
	fmt.Println("running component")
	return http.ListenAndServe(fmt.Sprintf(":%d", c.port), http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		fmt.Println(req.Method, " ", req.URL)
	}))
}
