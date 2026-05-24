package domain

import (
	"context"
	"time"
)

// MeterReading representa un registro puro del CSV o una fila de la base de datos.
// Como descubrimos antes, 'Value' es un acumulador incremental.
type MeterReading struct {
	ID        string    `json:"id"`
	MeterID   int       `json:"meter_id"`
	Value     float64   `json:"value"`
	Timestamp time.Time `json:"timestamp"`
}

// MeterData representa la serie de datos calculados y agrupados para un medidor específico.
// Esta estructura mapea exactamente lo que exige el PDF dentro del arreglo "data_graph".
type MeterData struct {
	MeterID            int       `json:"meter_id"`
	Address            string    `json:"address"`
	Active             []float64 `json:"active"`
	ReactiveInductive  []float64 `json:"reactive_inductive"`
	ReactiveCapacitive []float64 `json:"reactive_capacitive"`
	Exported           []float64 `json:"exported"`
}

// ConsumptionResponse es la estructura del JSON final que retornará nuestro endpoint.
type ConsumptionResponse struct {
	Period    []string    `json:"period"`
	DataGraph []MeterData `json:"data_graph"`
}

// ConsumptionRepository define el contrato que la capa de datos (infraestructura/SQL)
// debe implementar obligatoriamente. El dominio no sabe CÓMO se hace el query,
// solo sabe QUÉ datos necesita pedir.
type ConsumptionRepository interface {
	// SaveMultiple será utilizado por el script importador del CSV
	SaveMultiple(ctx context.Context, readings []MeterReading) error

	// GetReadings traerá los datos del periodo para que el caso de uso calcule las diferencias y sumas
	GetReadings(ctx context.Context, meterIDs []int, startDate, endDate time.Time) ([]MeterReading, error)
}
