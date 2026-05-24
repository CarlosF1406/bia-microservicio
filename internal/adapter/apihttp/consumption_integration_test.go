//go:build integration

package apihttp_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"bia-microservicio/internal/adapter/apihttp"
	"bia-microservicio/internal/adapter/repository" // 1. Importación real de tu capa de datos
	"bia-microservicio/internal/domain"
	"bia-microservicio/internal/usecase"

	_ "github.com/lib/pq"
)

type mockAddressIntegrationClient struct{}

func (m *mockAddressIntegrationClient) GetAddressByMeterID(ctx context.Context, meterID int) (string, error) {
	return "Dirección de Integración Real 456", nil
}

func TestStoreAndFetch_Integration(t *testing.T) {
	// 1. Conectar a la base de datos usando la variable de entorno o fallback
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		connStr = "postgres://postgres:postgres@localhost:5432/bia_db?sslmode=disable"
	}

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Fatalf("no se pudo conectar a la base de datos de integración: %v", err)
	}
	defer db.Close()

	// 2. Limpieza de tablas (Truncate) antes de correr el escenario
	ctx := context.Background()
	_, err = db.ExecContext(ctx, "TRUNCATE TABLE meter_readings RESTART IDENTITY CASCADE;")
	if err != nil {
		t.Fatalf("error limpiando la base de datos antes del test: %v", err)
	}

	// 3. Insertar registros reales en el MISMO DÍA para que el delta caiga en el índice 0
	insertQuery := `INSERT INTO meter_readings (id, meter_id, value, timestamp) VALUES ($1, $2, $3, $4);`

	uuid1 := "00000000-0000-0000-0000-000000000001"
	uuid2 := "00000000-0000-0000-0000-000000000002"

	// Lectura 1: 1 de Junio a las 08:00 AM
	_, err = db.ExecContext(ctx, insertQuery, uuid1, 1, 100.0, time.Date(2023, 6, 1, 8, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("error insertando data de prueba real: %v", err)
	}

	// Lectura 2: 1 de Junio a las 08:00 PM (Mismo día, 12 horas después)
	_, err = db.ExecContext(ctx, insertQuery, uuid2, 1, 150.0, time.Date(2023, 6, 1, 20, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("error insertando segunda data de prueba real: %v", err)
	}

	// 4. Inicializar las capas REALES utilizando tu constructor del repositorio
	repo := repository.NewPostgresConsumptionRepository(db) // 2. Modificado con tu constructor real
	addrClient := &mockAddressIntegrationClient{}
	uc := usecase.NewConsumptionUseCase(repo, addrClient)
	handler := apihttp.NewConsumptionHandler(uc)

	// 5. Simular la petición HTTP real al controlador
	req, err := http.NewRequest("GET", "/consumption?meters_ids=1&start_date=2023-06-01&end_date=2023-06-02&kind_period=daily", nil)
	if err != nil {
		t.Fatalf("error creando la petición HTTP de integración: %v", err)
	}

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// 6. Validaciones del ciclo completo
	if rr.Code != http.StatusOK {
		t.Errorf("codigo de estado esperado 200, obtenido %d. Body: %s", rr.Code, rr.Body.String())
	}

	var response domain.ConsumptionResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("error descodificando el JSON de respuesta real: %v", err)
	}

	if len(response.DataGraph) == 0 || len(response.DataGraph[0].Active) == 0 {
		t.Fatalf("la respuesta de la base de datos regresó arreglos vacíos")
	}

	// Comprobamos que el delta matemático (150.0 - 100.0 = 50.0) se procese de punta a punta correctamente
	gotActive := response.DataGraph[0].Active[0]
	if gotActive != 50.0 {
		t.Errorf("el cálculo de integración falló: esperado 50.0 del delta real de la DB, obtenido %f", gotActive)
	}
}
