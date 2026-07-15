package otel

import (
	"context"

	"github.com/nkonev/dcron"
)
import "go.opentelemetry.io/otel/trace"

func WithTracing(tracer trace.Tracer, spanName string, opts ...trace.SpanStartOption) dcron.JobOption {
	ot := &OtelTracing{
		tracer:   tracer,
		spanName: spanName,
		opts:     opts,
	}
	return dcron.WithTracing(ot.spanStarter, ot.spanFinisher)
}

type OtelTracing struct {
	tracer   trace.Tracer
	spanName string
	opts     []trace.SpanStartOption
}

func (i *OtelTracing) spanStarter(ctx context.Context) (context.Context, any) {
	return i.tracer.Start(ctx, i.spanName, i.opts...)
}

func (i *OtelTracing) spanFinisher(ctx context.Context, span any) {
	span.(trace.Span).End()
}
