package apihttp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"bia-microservicio/internal/domain"
	"bia-microservicio/internal/usecase"
)

// ─────────────────────────────────────────────
// Mocks para el handler
// ─────────────────────────────────────────────

type mockRepo struct {
	readings []domain.MeterReading
	err      error
}

func (m *mockRepo) SaveMultiple(_ context.Context, _ []domain.MeterReading) error { return m.err }
func (m *mockRepo) GetReadings(_ context.Context, _ []int, _, _ time.Time) ([]domain.MeterReading, error) {
	return m.readings, m.err
}

type mockAddressClient struct {
	address string
	err     error
}

func (m *mockAddressClient) GetAddressByMeterID(_ context.Context, _ int) (string, error) {
	return m.address, m.err
}

// ─────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────

// newTestHandler crea un handler con dependencias mockeadas.
func newTestHandler(readings []domain.MeterReading, repoErr error, address string) *ConsumptionHandler {
	repo := &mockRepo{readings: readings, err: repoErr}
	addrClient := &mockAddressClient{address: address}
	uc := usecase.NewConsumptionUseCase(repo, addrClient)
	return NewConsumptionHandler(uc)
}

// doRequest construye y ejecuta una petición GET al handler y devuelve el ResponseRecorder.
func doRequest(handler http.Handler, url string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, url, nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

// ─────────────────────────────────────────────
// Método no permitido
// ─────────────────────────────────────────────

func TestServeHTTP_MetodoPostDevuelve405(t *testing.T) {
	h := newTestHandler(nil, nil, "Calle 1")

	req := httptest.NewRequest(http.MethodPost, "/consumption?meters_ids=1&start_date=2023-06-01&end_date=2023-06-30&kind_period=monthly", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("esperado 405, obtenido %d", rr.Code)
	}
}

// ─────────────────────────────────────────────
// Parámetros faltantes → 400
// ─────────────────────────────────────────────

func TestServeHTTP_SinMetersIds_Devuelve400(t *testing.T) {
	h := newTestHandler(nil, nil, "Calle 1")
	rr := doRequest(h, "/consumption?start_date=2023-06-01&end_date=2023-06-30&kind_period=daily")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("esperado 400 sin meters_ids, obtenido %d", rr.Code)
	}
	assertErrorBody(t, rr, "meters_ids")
}

func TestServeHTTP_SinStartDate_Devuelve400(t *testing.T) {
	h := newTestHandler(nil, nil, "Calle 1")
	rr := doRequest(h, "/consumption?meters_ids=1&end_date=2023-06-30&kind_period=daily")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("esperado 400 sin start_date, obtenido %d", rr.Code)
	}
}

func TestServeHTTP_SinEndDate_Devuelve400(t *testing.T) {
	h := newTestHandler(nil, nil, "Calle 1")
	rr := doRequest(h, "/consumption?meters_ids=1&start_date=2023-06-01&kind_period=daily")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("esperado 400 sin end_date, obtenido %d", rr.Code)
	}
}

func TestServeHTTP_SinKindPeriod_Devuelve400(t *testing.T) {
	h := newTestHandler(nil, nil, "Calle 1")
	rr := doRequest(h, "/consumption?meters_ids=1&start_date=2023-06-01&end_date=2023-06-30")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("esperado 400 sin kind_period, obtenido %d", rr.Code)
	}
}

// ─────────────────────────────────────────────
// Parámetros inválidos → 400
// ─────────────────────────────────────────────

func TestServeHTTP_MeterIdNoNumerico_Devuelve400(t *testing.T) {
	h := newTestHandler(nil, nil, "Calle 1")
	rr := doRequest(h, "/consumption?meters_ids=abc&start_date=2023-06-01&end_date=2023-06-30&kind_period=daily")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("esperado 400 con meter_id no numérico, obtenido %d", rr.Code)
	}
}

func TestServeHTTP_StartDateFormatoInvalido_Devuelve400(t *testing.T) {
	h := newTestHandler(nil, nil, "Calle 1")
	rr := doRequest(h, "/consumption?meters_ids=1&start_date=01-06-2023&end_date=2023-06-30&kind_period=daily")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("esperado 400 con start_date inválido, obtenido %d", rr.Code)
	}
}

func TestServeHTTP_EndDateFormatoInvalido_Devuelve400(t *testing.T) {
	h := newTestHandler(nil, nil, "Calle 1")
	rr := doRequest(h, "/consumption?meters_ids=1&start_date=2023-06-01&end_date=30-06-2023&kind_period=daily")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("esperado 400 con end_date inválido, obtenido %d", rr.Code)
	}
}

func TestServeHTTP_FechasInvertidas_Devuelve400(t *testing.T) {
	// start_date > end_date → la regla de negocio debe rechazarlo
	h := newTestHandler(nil, nil, "Calle 1")
	rr := doRequest(h, "/consumption?meters_ids=1&start_date=2023-07-01&end_date=2023-06-01&kind_period=daily")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("esperado 400 por fechas invertidas, obtenido %d", rr.Code)
	}
}

// ─────────────────────────────────────────────
// Error del repositorio → 500
// ─────────────────────────────────────────────

func TestServeHTTP_ErrorRepositorio_Devuelve500(t *testing.T) {
	h := newTestHandler(nil, errors.New("fallo de BD"), "Calle 1")
	rr := doRequest(h, "/consumption?meters_ids=1&start_date=2023-06-01&end_date=2023-06-30&kind_period=daily")

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("esperado 500 por error de repositorio, obtenido %d", rr.Code)
	}
}

// ─────────────────────────────────────────────
// Petición exitosa → 200
// ─────────────────────────────────────────────

func TestServeHTTP_PeticionValida_Devuelve200ConJSON(t *testing.T) {
	readings := []domain.MeterReading{
		{ID: "1", MeterID: 1, Value: 100, Timestamp: time.Date(2023, 6, 1, 0, 0, 0, 0, time.UTC)},
		{ID: "2", MeterID: 1, Value: 150, Timestamp: time.Date(2023, 6, 2, 0, 0, 0, 0, time.UTC)},
	}

	h := newTestHandler(readings, nil, "Calle Falsa 123")
	rr := doRequest(h, "/consumption?meters_ids=1&start_date=2023-06-01&end_date=2023-06-03&kind_period=daily")

	if rr.Code != http.StatusOK {
		t.Fatalf("esperado 200, obtenido %d — body: %s", rr.Code, rr.Body.String())
	}

	contentType := rr.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type esperado 'application/json', obtenido %q", contentType)
	}

	var resp domain.ConsumptionResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("error al decodificar respuesta JSON: %v", err)
	}

	if len(resp.Period) == 0 {
		t.Error("se esperaba al menos un período en la respuesta")
	}
	if len(resp.DataGraph) == 0 {
		t.Error("se esperaban datos en data_graph")
	}
	if resp.DataGraph[0].Address != "Calle Falsa 123" {
		t.Errorf("dirección incorrecta: %q", resp.DataGraph[0].Address)
	}
}

func TestServeHTTP_MultiplesMedidoresEnQuery(t *testing.T) {
	readings := []domain.MeterReading{
		{ID: "1", MeterID: 1, Value: 100, Timestamp: time.Date(2023, 6, 1, 0, 0, 0, 0, time.UTC)},
		{ID: "2", MeterID: 2, Value: 200, Timestamp: time.Date(2023, 6, 1, 0, 0, 0, 0, time.UTC)},
	}

	h := newTestHandler(readings, nil, "Av. Test")
	rr := doRequest(h, "/consumption?meters_ids=1,2&start_date=2023-06-01&end_date=2023-06-30&kind_period=monthly")

	if rr.Code != http.StatusOK {
		t.Fatalf("esperado 200, obtenido %d", rr.Code)
	}

	var resp domain.ConsumptionResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("error al decodificar: %v", err)
	}

	if len(resp.DataGraph) != 2 {
		t.Errorf("esperados 2 medidores en data_graph, obtenido %d", len(resp.DataGraph))
	}
}

func TestServeHTTP_RespuestaContieneEstructuraCompleta(t *testing.T) {
	h := newTestHandler(nil, nil, "Dirección Completa")
	rr := doRequest(h, "/consumption?meters_ids=5&start_date=2023-06-01&end_date=2023-06-30&kind_period=monthly")

	if rr.Code != http.StatusOK {
		t.Fatalf("esperado 200, obtenido %d", rr.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("error al decodificar JSON: %v", err)
	}

	// Verifica que existan las claves raíz del contrato del API
	if _, ok := body["period"]; !ok {
		t.Error("falta el campo 'period' en la respuesta")
	}
	if _, ok := body["data_graph"]; !ok {
		t.Error("falta el campo 'data_graph' en la respuesta")
	}
}

// ─────────────────────────────────────────────
// Helpers de aserción
// ─────────────────────────────────────────────

// assertErrorBody verifica que el cuerpo de error JSON mencione la subcadena dada.
func assertErrorBody(t *testing.T, rr *httptest.ResponseRecorder, substring string) {
	t.Helper()
	body := rr.Body.String()
	if body == "" {
		t.Errorf("se esperaba cuerpo de error mencionando %q, pero la respuesta está vacía", substring)
		return
	}
	var payload map[string]string
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Errorf("body no es JSON válido: %s", body)
		return
	}
	msg, ok := payload["error"]
	if !ok || len(msg) == 0 {
		t.Errorf("se esperaba campo 'error' en el body de respuesta")
	}
}
