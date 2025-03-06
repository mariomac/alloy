package nriareceiver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	"github.com/mariomac/guara/pkg/casing"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"

	"github.com/grafana/alloy/internal/component/otelcol/receiver/nria/nriareceiver/internal/datapoint"
	"github.com/grafana/alloy/internal/component/otelcol/receiver/nria/nriareceiver/internal/nria"
)

type nriaReceiver struct {
	anress string
	config *Config
	params receiver.Settings

	nextTracesConsumer  consumer.Traces
	nextMetricsConsumer consumer.Metrics

	server    *http.Server
	tReceiver *receiverhelper.ObsReport
}

// Endpoint specifies an API endpoint definition.
type Endpoint struct {
	// Pattern specifies the API pattern, as registered by the HTTP handler.
	Pattern string

	// Handler specifies the http.Handler for this endpoint.
	Handler func(http.ResponseWriter, *http.Request)
}

// getEndpoints specifies the list of endpoints registered for the trace-agent API.
func (nr *nriaReceiver) getEndpoints() []Endpoint {
	endpoints := []Endpoint{
		{
			Pattern: "/",
			Handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
		},
	}

	// connect stuff
	endpoints = append(endpoints, Endpoint{
		Pattern: "/identity/v1/connect",
		Handler: nr.handleConnect,
	})
	if nr.nextTracesConsumer != nil {
		// TODO: manage traces here
	}

	if nr.nextMetricsConsumer != nil {
		endpoints = append(endpoints, []Endpoint{{
			Pattern: "/infra/v2/metrics/events/bulk",
			Handler: nr.handleV2BulkEvents,
		}, {
			Pattern: "/metrics/events/bulk",
			Handler: nr.handleV2BulkEvents,
		}}...)
	}
	return endpoints
}

func newNRIAReceiver(config *Config, params receiver.Settings) (component.Component, error) {
	instance, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{LongLivedCtx: false, ReceiverID: params.ID, Transport: "http", ReceiverCreateSettings: params})
	if err != nil {
		return nil, err
	}

	return &nriaReceiver{
		params: params,
		config: config,
		server: &http.Server{
			ReadTimeout: config.ReadTimeout,
		},
		tReceiver: instance,
	}, nil
}

func (nr *nriaReceiver) Start(ctx context.Context, host component.Host) error {
	ddmux := http.NewServeMux()
	endpoints := nr.getEndpoints()

	for _, e := range endpoints {
		ddmux.HandleFunc(e.Pattern, e.Handler)
	}

	var err error
	nr.server, err = nr.config.ServerConfig.ToServer(
		ctx,
		host,
		nr.params.TelemetrySettings,
		ddmux,
	)
	if err != nil {
		return fmt.Errorf("failed to create server definition: %w", err)
	}
	hln, err := nr.config.ServerConfig.ToListener(ctx)
	if err != nil {
		return fmt.Errorf("failed to create datadog listener: %w", err)
	}

	nr.anress = hln.Addr().String()

	go func() {
		if err := nr.server.Serve(hln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			componentstatus.ReportStatus(host, componentstatus.NewFatalErrorEvent(fmt.Errorf("error starting datadog receiver: %w", err)))
		}
	}()
	return nil
}

func (nr *nriaReceiver) Shutdown(ctx context.Context) (err error) {
	return nr.server.Shutdown(ctx)
}

// handleInfo handles incoming /info payloads.
func (nr *nriaReceiver) handleInfo(w http.ResponseWriter, _ *http.Request, infoResponse []byte) {
	_, err := fmt.Fprintf(w, "%s", infoResponse)
	if err != nil {
		nr.params.Logger.Error("Error writing /info endpoint response", zap.Error(err))
		http.Error(w, "Error writing /info endpoint response", http.StatusInternalServerError)
		return
	}
}

func (nr *nriaReceiver) handleConnect(writer http.ResponseWriter, request *http.Request) {
	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		nr.params.Logger.Error("Reading request body", zap.Error(err))
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	req := struct {
		Fingerprint nria.Fingerprint `json:"fingerprint"`
		Type        string           `json:"type"`
		Protocol    string           `json:"protocol"`
		EntityID    nria.EntityID    `json:"entityId,omitempty"`
	}{}

	if err := json.Unmarshal(body, &req); err != nil {
		nr.params.Logger.Error("parsing request body mapeao", zap.Error(err))
		writer.WriteHeader(http.StatusBadRequest)
		return
	}

	if request.Method == http.MethodPost {
		nr.params.Logger.Info("received connect fingerprint", zap.Any("connect", req))
	} else if request.Method == http.MethodPut && req.EntityID > 0 {
		nr.params.Logger.Info("received update fingerprint", zap.Any("reconnect", req))
	} else {
		writer.WriteHeader(http.StatusBadRequest)
	}

	writer.WriteHeader(http.StatusOK)
}

func (nr *nriaReceiver) handleV2BulkEvents(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	// TODO: handle compression
	datapoints, err := datapoint.ReadFrom(req.Body)
	if err != nil {
		nr.params.Logger.Error("Reading request body", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	obsCtx := nr.tReceiver.StartMetricsOp(req.Context())
	var metricsCount int
	defer func(metricsCount *int) {
		nr.tReceiver.EndMetricsOp(obsCtx, "nria", *metricsCount, err)
	}(&metricsCount)

	nr.params.Logger.Info("received datapoints", zap.Int("len", len(datapoints)))
	// use datapoint.convert to get data
	for _, dpg := range datapoints {
		// code below is highly unefficient as all the metrics and entities
		// are recreated on every iteration
		// TODO: cache everything

		// create resource with its metrics
		otelResource := pcommon.NewResource()
		otelResource.Attributes().PutStr("instance", strconv.Itoa(int(dpg.EntityID)))
		metrics := pmetric.NewMetrics()
		rmetrics := metrics.ResourceMetrics().AppendEmpty()
		otelResource.MoveTo(rmetrics.Resource())
		// create scope with its metrics
		otelScope := pcommon.NewInstrumentationScope()
		otelScope.SetName("nria")
		otelScope.SetVersion("0.0.1") // todo: set me
		smetrics := rmetrics.ScopeMetrics().AppendEmpty()
		otelScope.MoveTo(smetrics.Scope())

		metricPrefix := casing.CamelToDots(dpg.SampleName)

		// TODO: cache too!
		attrs := pcommon.NewMap()
		for k, v := range dpg.MetricAttrs {
			attrs.PutStr(k, v)
		}

		for _, dp := range dpg.DataPoints {
			// create actual metrics
			metric := smetrics.Metrics().AppendEmpty()
			metric.SetName(metricPrefix + "." + casing.CamelToDots(dp.Name))
			// we receive everything as an absolute value so we are treating them as gauges
			dps := metric.Gauge().DataPoints()
			dps.EnsureCapacity(1)
			ddp := dps.AppendEmpty()
			attrs.CopyTo(ddp.Attributes())
			ddp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(dp.TimestampSecs, 0)))
			ddp.SetDoubleValue(dp.Value)
		}

		err := nr.nextMetricsConsumer.ConsumeMetrics(req.Context(), metrics)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			nr.params.Logger.Error("metrics consumer errored out", zap.Error(err))
			return
		}
	}
	w.WriteHeader(http.StatusOK)
}
