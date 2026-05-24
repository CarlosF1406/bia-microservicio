package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"bia-microservicio/internal/domain"
)

// ─────────────────────────────────────────────
// Mocks
// ─────────────────────────────────────────────

// mockRepo implementa domain.ConsumptionRepository en memoria.
type mockRepo struct {
	readings []domain.MeterReading
	saveErr  error
	getErr   error
}

func (m *mockRepo) SaveMultiple(_ context.Context, _ []domain.MeterReading) error {
	return m.saveErr
}

func (m *mockRepo) GetReadings(_ context.Context, _ []int, _, _ time.Time) ([]domain.MeterReading, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.readings, nil
}

// mockAddressClient implementa domain.AddressServiceClient.
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

func newUC(repo domain.ConsumptionRepository, client domain.AddressServiceClient) *ConsumptionUseCase {
	return NewConsumptionUseCase(repo, client)
}

func date(y, m, d int) time.Time {
	return time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
}

func reading(meterID int, value float64, t time.Time) domain.MeterReading {
	return domain.MeterReading{
		ID:        "id",
		MeterID:   meterID,
		Value:     value,
		Timestamp: t,
	}
}

// ─────────────────────────────────────────────
// Execute – pruebas del método principal
// ─────────────────────────────────────────────

func TestExecute_ReturnsErrorWhenStartDateAfterEndDate(t *testing.T) {
	uc := newUC(&mockRepo{}, &mockAddressClient{})

	start := date(2023, 7, 10)
	end := date(2023, 6, 1)

	_, err := uc.Execute(context.Background(), []int{1}, start, end, "monthly")
	if err == nil {
		t.Fatal("se esperaba error por fechas invertidas, pero no se obtuvo ninguno")
	}
}

func TestExecute_ReturnsErrorWhenRepositoryFails(t *testing.T) {
	repo := &mockRepo{getErr: errors.New("error de base de datos")}
	uc := newUC(repo, &mockAddressClient{address: "Calle 1"})

	_, err := uc.Execute(context.Background(), []int{1}, date(2023, 6, 1), date(2023, 6, 30), "daily")
	if err == nil {
		t.Fatal("se esperaba error por fallo del repositorio")
	}
}

func TestExecute_UsaFallbackDireccionCuandoClienteFalla(t *testing.T) {
	repo := &mockRepo{readings: []domain.MeterReading{}}
	addrClient := &mockAddressClient{err: errors.New("servicio no disponible")}
	uc := newUC(repo, addrClient)

	resp, err := uc.Execute(context.Background(), []int{1}, date(2023, 6, 1), date(2023, 6, 3), "daily")
	if err != nil {
		t.Fatalf("no se esperaba error, obtuvo: %v", err)
	}

	if resp.DataGraph[0].Address != "Dirección no disponible" {
		t.Errorf("dirección fallback incorrecta: %q", resp.DataGraph[0].Address)
	}
}

func TestExecute_CalculaConsumosDiarios(t *testing.T) {
	// Medidor 1 con lecturas acumuladas: la energía consumida por día es la diferencia.
	start := date(2023, 6, 1)
	end := date(2023, 6, 3)

	r := []domain.MeterReading{
		reading(1, 100.0, date(2023, 6, 1)),
		reading(1, 110.0, date(2023, 6, 2)), // delta = 10
		reading(1, 125.0, date(2023, 6, 3)), // delta = 15
	}

	repo := &mockRepo{readings: r}
	addrClient := &mockAddressClient{address: "Calle Falsa 123"}
	uc := newUC(repo, addrClient)

	resp, err := uc.Execute(context.Background(), []int{1}, start, end, "daily")
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	active := resp.DataGraph[0].Active
	if len(active) != 3 {
		t.Fatalf("se esperaban 3 periodos, se obtuvo %d", len(active))
	}

	// El delta de 10 pertenece al día 2 (índice 1), el de 15 al día 3 (índice 2).
	if active[1] != 10.0 {
		t.Errorf("consumo día 2: esperado 10, obtenido %.2f", active[1])
	}
	if active[2] != 15.0 {
		t.Errorf("consumo día 3: esperado 15, obtenido %.2f", active[2])
	}
}

func TestExecute_IgnoraDeltasNegativos(t *testing.T) {
	// Simula el reinicio de un medidor (valor cae a 0).
	start := date(2023, 6, 1)
	end := date(2023, 6, 3)

	r := []domain.MeterReading{
		reading(1, 200.0, date(2023, 6, 1)),
		reading(1, 0.0, date(2023, 6, 2)),  // medidor reiniciado → delta negativo, se ignora
		reading(1, 10.0, date(2023, 6, 3)), // delta = 10 (sobre el 0)
	}

	repo := &mockRepo{readings: r}
	uc := newUC(repo, &mockAddressClient{address: "Av. Siempre Viva"})

	resp, err := uc.Execute(context.Background(), []int{1}, start, end, "daily")
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	active := resp.DataGraph[0].Active
	// Día 2 (índice 1): delta negativo ignorado → 0
	if active[1] != 0.0 {
		t.Errorf("se esperaba 0 para delta negativo, obtenido %.2f", active[1])
	}
	// Día 3 (índice 2): 10
	if active[2] != 10.0 {
		t.Errorf("consumo día 3: esperado 10, obtenido %.2f", active[2])
	}
}

func TestExecute_RetornaArraysVaciosConMenosDeDosMediciones(t *testing.T) {
	// Con una sola lectura no hay diferencia que calcular.
	r := []domain.MeterReading{
		reading(1, 100.0, date(2023, 6, 1)),
	}

	repo := &mockRepo{readings: r}
	uc := newUC(repo, &mockAddressClient{address: "Calle 1"})

	resp, err := uc.Execute(context.Background(), []int{1}, date(2023, 6, 1), date(2023, 6, 3), "daily")
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	for _, v := range resp.DataGraph[0].Active {
		if v != 0.0 {
			t.Errorf("se esperaban todos los consumos en 0, obtenido %.2f", v)
		}
	}
}

func TestExecute_MultiplesMedidores(t *testing.T) {
	start := date(2023, 6, 1)
	end := date(2023, 6, 2)

	r := []domain.MeterReading{
		reading(1, 100.0, date(2023, 6, 1)),
		reading(1, 120.0, date(2023, 6, 2)), // delta = 20
		reading(2, 50.0, date(2023, 6, 1)),
		reading(2, 80.0, date(2023, 6, 2)), // delta = 30
	}

	repo := &mockRepo{readings: r}
	uc := newUC(repo, &mockAddressClient{address: "Dirección X"})

	resp, err := uc.Execute(context.Background(), []int{1, 2}, start, end, "daily")
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	if len(resp.DataGraph) != 2 {
		t.Fatalf("se esperaban 2 medidores, obtenido %d", len(resp.DataGraph))
	}

	// Buscamos por meter_id para no depender del orden de iteración del mapa interno
	find := func(id int) domain.MeterData {
		for _, d := range resp.DataGraph {
			if d.MeterID == id {
				return d
			}
		}
		t.Fatalf("medidor %d no encontrado en la respuesta", id)
		return domain.MeterData{}
	}

	if find(1).Active[1] != 20.0 {
		t.Errorf("medidor 1: esperado 20, obtenido %.2f", find(1).Active[1])
	}
	if find(2).Active[1] != 30.0 {
		t.Errorf("medidor 2: esperado 30, obtenido %.2f", find(2).Active[1])
	}
}

func TestExecute_TodosLosArraysReactivosSonCero(t *testing.T) {
	// Según el dataset, reactive_inductive, reactive_capacitive y exported siempre son 0.
	r := []domain.MeterReading{
		reading(1, 100.0, date(2023, 6, 1)),
		reading(1, 200.0, date(2023, 6, 2)),
	}

	uc := newUC(&mockRepo{readings: r}, &mockAddressClient{address: "Av. 1"})

	resp, err := uc.Execute(context.Background(), []int{1}, date(2023, 6, 1), date(2023, 6, 2), "daily")
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	md := resp.DataGraph[0]
	for i, v := range md.ReactiveInductive {
		if v != 0.0 {
			t.Errorf("ReactiveInductive[%d] debería ser 0, obtenido %.2f", i, v)
		}
	}
	for i, v := range md.ReactiveCapacitive {
		if v != 0.0 {
			t.Errorf("ReactiveCapacitive[%d] debería ser 0, obtenido %.2f", i, v)
		}
	}
	for i, v := range md.Exported {
		if v != 0.0 {
			t.Errorf("Exported[%d] debería ser 0, obtenido %.2f", i, v)
		}
	}
}

// ─────────────────────────────────────────────
// generatePeriods
// ─────────────────────────────────────────────

func TestGeneratePeriods_Daily(t *testing.T) {
	uc := newUC(&mockRepo{}, &mockAddressClient{})
	start := date(2023, 6, 1)
	end := date(2023, 6, 3)

	periods := uc.generatePeriods(start, end, "daily")

	expected := []string{"JUN 1", "JUN 2", "JUN 3"}
	if len(periods) != len(expected) {
		t.Fatalf("esperado %d periodos, obtenido %d", len(expected), len(periods))
	}
	for i, p := range periods {
		if p != expected[i] {
			t.Errorf("periodo[%d]: esperado %q, obtenido %q", i, expected[i], p)
		}
	}
}

func TestGeneratePeriods_Weekly(t *testing.T) {
	uc := newUC(&mockRepo{}, &mockAddressClient{})
	start := date(2023, 6, 1)
	end := date(2023, 6, 14)

	periods := uc.generatePeriods(start, end, "weekly")

	if len(periods) != 2 {
		t.Fatalf("esperado 2 semanas, obtenido %d", len(periods))
	}
	if periods[0] != "JUN 1 - JUN 7" {
		t.Errorf("semana 1: esperado %q, obtenido %q", "JUN 1 - JUN 7", periods[0])
	}
	if periods[1] != "JUN 8 - JUN 14" {
		t.Errorf("semana 2: esperado %q, obtenido %q", "JUN 8 - JUN 14", periods[1])
	}
}

func TestGeneratePeriods_Monthly(t *testing.T) {
	uc := newUC(&mockRepo{}, &mockAddressClient{})
	start := date(2023, 6, 1)
	end := date(2023, 8, 31)

	periods := uc.generatePeriods(start, end, "monthly")

	expected := []string{"JUN 2023", "JUL 2023", "AUG 2023"}
	if len(periods) != len(expected) {
		t.Fatalf("esperado %d periodos, obtenido %d: %v", len(expected), len(periods), periods)
	}
	for i, p := range periods {
		if p != expected[i] {
			t.Errorf("mes[%d]: esperado %q, obtenido %q", i, expected[i], p)
		}
	}
}

func TestGeneratePeriods_UnknownKindRetornaVacio(t *testing.T) {
	uc := newUC(&mockRepo{}, &mockAddressClient{})
	periods := uc.generatePeriods(date(2023, 6, 1), date(2023, 6, 5), "anual")
	if len(periods) != 0 {
		t.Errorf("se esperaba slice vacío para kind desconocido, obtenido %v", periods)
	}
}

func TestGeneratePeriods_MismoDiaRetornaUnPeriodo(t *testing.T) {
	uc := newUC(&mockRepo{}, &mockAddressClient{})
	d := date(2023, 6, 15)
	periods := uc.generatePeriods(d, d, "daily")
	if len(periods) != 1 {
		t.Errorf("se esperaba 1 período para mismo día, obtenido %d", len(periods))
	}
}

// ─────────────────────────────────────────────
// filterReadingsByMeter
// ─────────────────────────────────────────────

func TestFilterReadingsByMeter(t *testing.T) {
	uc := newUC(&mockRepo{}, &mockAddressClient{})

	all := []domain.MeterReading{
		{MeterID: 1, Value: 10},
		{MeterID: 2, Value: 20},
		{MeterID: 1, Value: 30},
	}

	filtered := uc.filterReadingsByMeter(all, 1)
	if len(filtered) != 2 {
		t.Fatalf("esperado 2 lecturas para medidor 1, obtenido %d", len(filtered))
	}
	for _, r := range filtered {
		if r.MeterID != 1 {
			t.Errorf("lectura con MeterID %d no debería estar en el resultado", r.MeterID)
		}
	}
}

func TestFilterReadingsByMeter_MedidorInexistente(t *testing.T) {
	uc := newUC(&mockRepo{}, &mockAddressClient{})

	all := []domain.MeterReading{{MeterID: 1, Value: 10}}
	filtered := uc.filterReadingsByMeter(all, 99)
	if len(filtered) != 0 {
		t.Errorf("se esperaba slice vacío, obtenido %d elementos", len(filtered))
	}
}

// ─────────────────────────────────────────────
// getPeriodIndex
// ─────────────────────────────────────────────

func TestGetPeriodIndex_Daily(t *testing.T) {
	uc := newUC(&mockRepo{}, &mockAddressClient{})
	start := date(2023, 6, 1)

	cases := []struct {
		t        time.Time
		expected int
	}{
		{date(2023, 6, 1), 0},
		{date(2023, 6, 5), 4},
		{date(2023, 6, 10), 9},
	}

	for _, tc := range cases {
		got := uc.getPeriodIndex(tc.t, start, "daily")
		if got != tc.expected {
			t.Errorf("getPeriodIndex(%v) daily: esperado %d, obtenido %d", tc.t, tc.expected, got)
		}
	}
}

func TestGetPeriodIndex_Weekly(t *testing.T) {
	uc := newUC(&mockRepo{}, &mockAddressClient{})
	start := date(2023, 6, 1)

	cases := []struct {
		t        time.Time
		expected int
	}{
		{date(2023, 6, 1), 0},  // semana 1
		{date(2023, 6, 7), 0},  // último día semana 1
		{date(2023, 6, 8), 1},  // inicio semana 2
		{date(2023, 6, 14), 1}, // último día semana 2
	}

	for _, tc := range cases {
		got := uc.getPeriodIndex(tc.t, start, "weekly")
		if got != tc.expected {
			t.Errorf("getPeriodIndex(%v) weekly: esperado %d, obtenido %d", tc.t, tc.expected, got)
		}
	}
}

func TestGetPeriodIndex_Monthly(t *testing.T) {
	uc := newUC(&mockRepo{}, &mockAddressClient{})
	start := date(2023, 6, 1)

	cases := []struct {
		t        time.Time
		expected int
	}{
		{date(2023, 6, 15), 0},
		{date(2023, 7, 1), 1},
		{date(2023, 12, 31), 6},
	}

	for _, tc := range cases {
		got := uc.getPeriodIndex(tc.t, start, "monthly")
		if got != tc.expected {
			t.Errorf("getPeriodIndex(%v) monthly: esperado %d, obtenido %d", tc.t, tc.expected, got)
		}
	}
}

func TestGetPeriodIndex_KindDesconocidoRetornaMenosUno(t *testing.T) {
	uc := newUC(&mockRepo{}, &mockAddressClient{})
	got := uc.getPeriodIndex(date(2023, 6, 1), date(2023, 6, 1), "anual")
	if got != -1 {
		t.Errorf("esperado -1 para kind desconocido, obtenido %d", got)
	}
}

// ─────────────────────────────────────────────
// Pruebas avanzadas de ordenamiento y agregación masiva
// ─────────────────────────────────────────────

func TestExecute_OrdenaCronologicamenteYAcumulaSemanas(t *testing.T) {
	// 1. Pasamos lecturas DESORDENADAS cronológicamente para obligar a que funcione el sort.Slice
	// 2. Colocamos mediciones que caen en la misma semana para comprobar la acumulación incremental
	start := date(2023, 6, 1) // Semana 1: JUN 1 al JUN 7
	end := date(2023, 6, 14)  // Semana 2: JUN 8 al JUN 14

	r := []domain.MeterReading{
		reading(1, 150.0, date(2023, 6, 10)), // Lectura intermedia (Semana 2)
		reading(1, 100.0, date(2023, 6, 1)),  // Lectura inicial base (Semana 1)
		reading(1, 120.0, date(2023, 6, 5)),  // Cae en Semana 1. Delta con base = 20
		reading(1, 130.0, date(2023, 6, 7)),  // Cae en Semana 1. Delta con ant = 10. Total Semana 1 = 30
		reading(1, 185.0, date(2023, 6, 12)), // Lectura final (Semana 2). Delta con inter = 35. Total Semana 2 = (150-130)+(185-150) = 55
	}

	repo := &mockRepo{readings: r}
	uc := newUC(repo, &mockAddressClient{address: "Calle de las Pruebas"})

	resp, err := uc.Execute(context.Background(), []int{1}, start, end, "weekly")
	if err != nil {
		t.Fatalf("error inesperado en procesamiento semanal: %v", err)
	}

	active := resp.DataGraph[0].Active
	if len(active) != 2 {
		t.Fatalf("se esperaban 2 periodos semanales, se obtuvieron %d", len(active))
	}

	// Semana 1 (Índice 0): acumulado de deltas desde 100 hasta 130 -> 30.0
	if active[0] != 30.0 {
		t.Errorf("Acumulación Semana 1 incorrecta: esperado 30.0, obtenido %.2f", active[0])
	}

	// Semana 2 (Índice 1): acumulado de deltas desde 130 hasta 185 -> 55.0
	if active[1] != 55.0 {
		t.Errorf("Acumulación Semana 2 incorrecta (posible fallo de ordenamiento): esperado 55.0, obtenido %.2f", active[1])
	}
}
