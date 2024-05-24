package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/exporters/zipkin"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

var tracer trace.Tracer

func initTracer() {
	exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		log.Fatalf("failed to initialize stdouttrace exporter: %v", err)
	}

	zipkinExporter, err := zipkin.New(
		"http://zipkin:9411/api/v2/spans",
	)
	if err != nil {
		log.Fatalf("failed to initialize zipkin exporter: %v", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithBatcher(zipkinExporter),
		sdktrace.WithResource(resource.NewWithAttributes("", attribute.String("service.name", "servicea"))),
	)

	otel.SetTracerProvider(tp)
	tracer = tp.Tracer("servicea")
}

func validateCep(cep string) bool {
	if len(cep) != 8 {
		return false
	}

	for _, ch := range cep {
		if ch < '0' || ch > '9' {
			return false
		}
	}

	return true
}

func handler(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "handler")
	defer span.End()

	var request struct {
		Cep string `json:"cep"`
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "could not read request body", http.StatusInternalServerError)
		return
	}

	if err := json.Unmarshal(body, &request); err != nil {
		http.Error(w, "invalid request format", http.StatusBadRequest)
		return
	}

	if !validateCep(request.Cep) {
		http.Error(w, "invalid zipcode", http.StatusUnprocessableEntity)
		return
	}

	req, _ := http.NewRequestWithContext(ctx, "POST", "http://serviceb:8081/cep", strings.NewReader(string(body)))
	client := http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}

	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "could not communicate with service b", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	responseBody, _ := io.ReadAll(resp.Body)
	w.WriteHeader(resp.StatusCode)
	w.Write(responseBody)
}

func main() {
	initTracer()
	mux := http.NewServeMux()
	mux.Handle("/", otelhttp.NewHandler(http.HandlerFunc(handler), "handler"))
	fmt.Println("Serviço A disponível em :8080")
	http.ListenAndServe(":8080", mux)
}
