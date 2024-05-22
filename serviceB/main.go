package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"

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
		sdktrace.WithResource(resource.NewWithAttributes("", attribute.String("service.name", "serviceb"))),
	)
	otel.SetTracerProvider(tp)
	tracer = tp.Tracer("serviceb")
}

type ViaCepResponse struct {
	Error      bool   `json:"erro"`
	Localidade string `json:"localidade"`
}

type NominatimResponse struct {
	Lat float64 `json:"lat,string"`
	Lon float64 `json:"lon,string"`
}

type OpenMeteoResponse struct {
	CurrentWeather struct {
		Temperature float64 `json:"temperature"`
	} `json:"current_weather"`
}

func getCity(cep string) ViaCepResponse {
	var data ViaCepResponse
	url := fmt.Sprintf("https://viacep.com.br/ws/%s/json/", cep)

	resp, err := http.Get(url)
	if err != nil {
		return ViaCepResponse{}
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ViaCepResponse{}
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return ViaCepResponse{}
	}

	return data
}

func getCoordinates(location string) (float64, float64, error) {
	url := fmt.Sprintf("https://nominatim.openstreetmap.org/search?format=json&q=%s", url.QueryEscape(location))

	resp, err := http.Get(url)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, 0, fmt.Errorf("non-200 response from geocoding service: %s, body: %s", resp.Status, body)
	}

	var data []NominatimResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return 0, 0, err
	}
	if len(data) == 0 {
		return 0, 0, fmt.Errorf("no results found for location")
	}

	return data[0].Lat, data[0].Lon, nil
}

func getTemperature(latitude, longitude float64) (float64, error) {
	url := fmt.Sprintf("https://api.open-meteo.com/v1/forecast?latitude=%f&longitude=%f&current_weather=true", latitude, longitude)

	log.Printf("URL Request: %s", url)

	resp, err := http.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Response Body: %s", body)
		return 0, fmt.Errorf("non-200 response from Open Meteo: %s, body: %s", resp.Status, body)
	}

	var data OpenMeteoResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return 0, err
	}

	return data.CurrentWeather.Temperature, nil
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

func temperatureHandler(w http.ResponseWriter, r *http.Request) {
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

	city := getCity(request.Cep)
	if city.Error {
		http.Error(w, "can not find zipcode", http.StatusNotFound)
		return
	}

	lat, lon, err := getCoordinates(city.Localidade)
	if err != nil {
		http.Error(w, "can not find zipcode", http.StatusNotFound)
		return
	}

	tempC, err := getTemperature(lat, lon)
	if err != nil {
		http.Error(w, "could not get temperature", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"city":   city.Localidade,
		"temp_C": tempC,
		"temp_F": tempC*1.8 + 32,
		"temp_K": tempC + 273,
	}

	respBody, _ := json.Marshal(response)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(respBody)
}

func main() {
	initTracer()

	mux := http.NewServeMux()
	mux.Handle("/cep", otelhttp.NewHandler(http.HandlerFunc(temperatureHandler), "temperatureHandler"))

	fmt.Println("Serviço B disponível em :8081")
	http.ListenAndServe(":8081", mux)
}
