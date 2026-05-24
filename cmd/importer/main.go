package main

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"time"

	_ "github.com/lib/pq" // Driver de PostgreSQL
)

func main() {
	// 1. Configurar la conexión a la base de datos de manera dinámica
	dbHost := getEnv("DB_HOST", "localhost")
	dbPort := getEnv("DB_PORT", "5432")
	dbUser := getEnv("DB_USER", "postgres")
	dbPass := getEnv("DB_PASSWORD", "postgres")
	dbName := getEnv("DB_NAME", "bia_db")

	// Construir la URL de conexión dinámicamente
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		dbUser, dbPass, dbHost, dbPort, dbName)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Error abriendo conexión a la BD: %v", err)
	}
	defer db.Close()

	// 2. Abrir el archivo CSV
	csvPath := os.Getenv("CSV_FILE_PATH")
	if csvPath == "" {
		// Fallback por si se ejecuta el script sin configurar el .env
		csvPath = "data/test_bia.csv"
	}

	log.Printf("Abriendo archivo CSV desde la ruta: %s", csvPath)
	csvFile, err := os.Open(csvPath)
	if err != nil {
		log.Fatalf("Error crítico al abrir el archivo CSV: %v. Asegúrate de configurar correctamente CSV_FILE_PATH en tu .env", err)
	}
	defer csvFile.Close()

	reader := csv.NewReader(csvFile)

	fmt.Println("Iniciando la importación de datos...")
	startTime := time.Now()

	// 3. Iniciar una transacción SQL
	tx, err := db.Begin()
	if err != nil {
		log.Fatalf("Error al iniciar la transacción: %v", err)
	}

	// Preparamos el query para inserción en lote (bulk insert) eficiente
	stmt, err := tx.Prepare("INSERT INTO meter_readings (id, meter_id, value, timestamp) VALUES ($1, $2, $3, $4)")
	if err != nil {
		tx.Rollback()
		log.Fatalf("Error preparando el statement: %v", err)
	}
	defer stmt.Close()

	rowCount := 0

	// 4. Leer el CSV fila por fila
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break // Llegamos al final del archivo
		}
		if err != nil {
			tx.Rollback()
			log.Fatalf("Error leyendo fila del CSV: %v", err)
		}

		// Parsear campos según el orden analizado
		id := record[0]

		meterID, err := strconv.Atoi(record[1])
		if err != nil {
			log.Printf("Fila omitida por MeterID inválido: %v", record)
			continue
		}

		value, err := strconv.ParseFloat(record[2], 64)
		if err != nil {
			log.Printf("Fila omitida por valor numérico inválido: %v", record)
			continue
		}

		// El formato en el CSV es como: 2023-07-04 12:59:00+00
		layout := "2006-01-02 15:04:05-07"
		timestamp, err := time.Parse(layout, record[3])
		if err != nil {
			log.Printf("Fila omitida por timestamp inválido (%s): %v", record[3], err)
			continue
		}

		// Ejecutar el insert dentro de la transacción
		_, err = stmt.Exec(id, meterID, value, timestamp)
		if err != nil {
			tx.Rollback()
			log.Fatalf("Error insertando registro en la BD: %v", err)
		}

		rowCount++
	}

	// 5. Confirmar los cambios en la base de datos
	err = tx.Commit()
	if err != nil {
		log.Fatalf("Error al hacer commit de la transacción: %v", err)
	}

	fmt.Printf("¡Importación exitosa! Se insertaron %d registros en %v\n", rowCount, time.Since(startTime))
}

// getEnv es una función auxiliar para manejar valores por defecto si la variable de entorno no existe.
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if len(value) == 0 {
		return defaultValue
	}
	return value
}
