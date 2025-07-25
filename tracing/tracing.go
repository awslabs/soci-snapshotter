/*
   Copyright The Soci Snapshotter Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package tracing

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/trace"
)

const (
	sdkDisabledEnv        = "OTEL_SDK_DISABLED"
	otlpEndpointEnv       = "OTEL_EXPORTER_OTLP_ENDPOINT"
	otlpTracesEndpointEnv = "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"
	otlpProtocolEnv       = "OTEL_EXPORTER_OTLP_PROTOCOL"
	otlpTracesProtocolEnv = "OTEL_EXPORTER_OTLP_TRACES_PROTOCOL"
	otelTracesExporterEnv = "OTEL_TRACES_EXPORTER"
	otelServiceNameEnv    = "OTEL_SERVICE_NAME"
	defaultServiceName    = "soci-snapshotter"
)

func Init(ctx context.Context) (func(context.Context) error, error) {
	exp, err := newExporter(ctx)
	if err != nil {
		return nil, err
	}
	return setupTracer(exp), nil
}

func IsDisabled() (bool, error) {
	v := os.Getenv(sdkDisabledEnv)
	if v != "" {
		disabled, err := strconv.ParseBool(v)
		if err != nil {
			return true, fmt.Errorf("invalid value for env %s: %w", sdkDisabledEnv, err)
		}
		if disabled {
			return true, nil
		}
	}

	// not configuring an endpoint is considered as disabling tracing
	if os.Getenv(otlpEndpointEnv) == "" && os.Getenv(otlpTracesEndpointEnv) == "" {
		return true, nil
	}
	return false, nil
}

func newExporter(ctx context.Context) (*otlptrace.Exporter, error) {
	// Like containerd, "otlp" is the only supported traces exporter
	if v := os.Getenv(otelTracesExporterEnv); v != "" && v != "otlp" {
		return nil, fmt.Errorf("unsupported traces exporter %q", v)
	}

	v := os.Getenv(otlpTracesProtocolEnv)
	if v == "" {
		v = os.Getenv(otlpProtocolEnv)
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	switch v {
	case "", "http/protobuf":
		return otlptracehttp.New(ctx)
	case "grpc":
		return otlptracegrpc.New(ctx)
	default:
		// Like containerd, other protocols such as "http/json" are not supported.
		return nil, fmt.Errorf("unsupported OpenTelemetry protocol %q", v)
	}
}

func setupTracer(exp *otlptrace.Exporter) func(context.Context) error {
	if os.Getenv(otelServiceNameEnv) == "" {
		os.Setenv(otelServiceNameEnv, defaultServiceName)
	}

	provider := trace.NewTracerProvider(trace.WithBatcher(exp))
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		if err := provider.Shutdown(ctx); err != nil {
			return fmt.Errorf("failed to shutdown trace provider")
		}
		return nil
	}
}
