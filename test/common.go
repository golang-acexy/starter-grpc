package test

import "github.com/acexy/golang-toolkit/sys/routine"

type traceIdSupplier struct {
	rt *routine.ThreadLocal[string]
}

func (t *traceIdSupplier) SetTraceId(traceId string) {
	t.rt.Set(traceId)
}

func (t *traceIdSupplier) GetTraceId() string {
	return t.rt.Get()
}

var supplier = &traceIdSupplier{
	rt: routine.NewTraceIdThreadLocal(nil),
}

func GetTraceIdSupplier() *traceIdSupplier {
	return supplier
}
