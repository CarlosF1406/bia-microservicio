package repository

import (
	"context"
	"database/sql"
	"time"

	"bia-microservicio/internal/domain"

	"github.com/lib/pq" // Requerido para mapear slices de Go a arrays de Postgres
)

// PostgresConsumptionRepository implementa la interfaz domain.ConsumptionRepository
type PostgresConsumptionRepository struct {
	db *sql.DB
}

// NewPostgresConsumptionRepository es el constructor para el repositorio de Postgres.
func NewPostgresConsumptionRepository(db *sql.DB) *PostgresConsumptionRepository {
	return &PostgresConsumptionRepository{
		db: db,
	}
}

// GetReadings busca todas las lecturas de los medidores solicitados dentro del rango de fechas.
func (r *PostgresConsumptionRepository) GetReadings(ctx context.Context, meterIDs []int, startDate, endDate time.Time) ([]domain.MeterReading, error) {
	// Definimos el query SQL.
	// El operador = ANY($1) equivale a un WHERE meter_id IN (...), pero es mucho más limpio
	// de parametrizar en Go cuando el tamaño del slice es dinámico.
	query := `
		SELECT id, meter_id, value, timestamp
		FROM meter_readings
		WHERE meter_id = ANY($1)
		  AND timestamp >= $2
		  AND timestamp <= $3
		ORDER BY timestamp ASC;
	`

	// Ejecutamos la consulta pasando el contexto para soportar timeouts/cancelaciones
	rows, err := r.db.QueryContext(ctx, query, pq.Array(meterIDs), startDate, endDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var readings []domain.MeterReading

	// Iteramos sobre los registros devueltos por la base de datos
	for rows.Next() {
		var reading domain.MeterReading

		// Escaneamos las columnas directamente en nuestra estructura del dominio
		err := rows.Scan(
			&reading.ID,
			&reading.MeterID,
			&reading.Value,
			&reading.Timestamp,
		)
		if err != nil {
			return nil, err
		}

		readings = append(readings, reading)
	}

	// Buena práctica: Verificar si el ciclo terminó por un error interno de lectura
	if err = rows.Err(); err != nil {
		return nil, err
	}

	return readings, nil
}

// SaveMultiple inserta un lote de lecturas en la base de datos usando una transacción de Postgres.
func (r *PostgresConsumptionRepository) SaveMultiple(ctx context.Context, readings []domain.MeterReading) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	stmt, err := tx.PrepareContext(ctx, "INSERT INTO meter_readings (id, meter_id, value, timestamp) VALUES ($1, $2, $3, $4)")
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()

	for _, reading := range readings {
		_, err := stmt.ExecContext(ctx, reading.ID, reading.MeterID, reading.Value, reading.Timestamp)
		if err != nil {
			_ = tx.Rollback()
			return err
		}
	}

	return tx.Commit()
}
