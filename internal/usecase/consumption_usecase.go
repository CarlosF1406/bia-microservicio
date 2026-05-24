package usecase

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"bia-microservicio/internal/domain"
)

// ConsumptionUseCase maneja la orquestación de la lógica de negocio de consumos.
type ConsumptionUseCase struct {
	repo          domain.ConsumptionRepository
	addressClient domain.AddressServiceClient
}

// NewConsumptionUseCase es el constructor que inyecta las dependencias necesarias.
func NewConsumptionUseCase(repo domain.ConsumptionRepository, addressClient domain.AddressServiceClient) *ConsumptionUseCase {
	return &ConsumptionUseCase{
		repo:          repo,
		addressClient: addressClient,
	}
}

// Execute procesa la solicitud, calcula los consumos reales y agrupa por el periodo solicitado.
func (uc *ConsumptionUseCase) Execute(ctx context.Context, meterIDs []int, startDate, endDate time.Time, kindPeriod string) (domain.ConsumptionResponse, error) {
	// 1. Validar reglas de negocio básicas
	if startDate.After(endDate) {
		return domain.ConsumptionResponse{}, errors.New("start_date cannot be after end_date")
	}

	// 2. Traer las lecturas crudas desde el repositorio
	readings, err := uc.repo.GetReadings(ctx, meterIDs, startDate, endDate)
	if err != nil {
		return domain.ConsumptionResponse{}, err
	}

	// 3. Generar la lista de etiquetas de tiempo ("period") según el kindPeriod (daily, weekly, monthly)
	periods := uc.generatePeriods(startDate, endDate, kindPeriod)

	// 4. Procesar la data por cada medidor solicitado
	var dataGraph []domain.MeterData
	for _, meterID := range meterIDs {
		// A. Obtener la dirección física usando el cliente externo
		address, err := uc.addressClient.GetAddressByMeterID(ctx, meterID)
		if err != nil {
			address = "Dirección no disponible"
		}

		// B. Filtrar las lecturas que pertenecen a ESTE medidor en específico
		meterReadings := uc.filterReadingsByMeter(readings, meterID)

		// C. CALCULAR EL CONSUMO REAL Y AGRUPARLO
		activeConsumptions := uc.calculateAggregatedConsumption(meterReadings, periods, kindPeriod, startDate)

		// D. Armar el objeto para la gráfica asegurando que todos los arreglos tengan el mismo tamaño
		meterData := domain.MeterData{
			MeterID:            meterID,
			Address:            address,
			Active:             activeConsumptions,
			ReactiveInductive:  make([]float64, len(periods)), // Inicializados en 0 por requerimiento del dataset
			ReactiveCapacitive: make([]float64, len(periods)), // Inicializados en 0 por requerimiento del dataset
			Exported:           make([]float64, len(periods)), // Inicializados en 0 por requerimiento del dataset
		}

		dataGraph = append(dataGraph, meterData)
	}

	// 5. Construir y retornar la respuesta final estructurada
	return domain.ConsumptionResponse{
		Period:    periods,
		DataGraph: dataGraph,
	}, nil
}

// generatePeriods genera el arreglo de etiquetas de texto según el formato exigido en el PDF.
func (uc *ConsumptionUseCase) generatePeriods(start, end time.Time, kind string) []string {
	var periods []string

	// Normalizamos eliminando horas para trabajar con fechas puras
	start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
	end = time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, end.Location())

	switch strings.ToLower(kind) {
	case "daily":
		for current := start; !current.After(end); current = current.AddDate(0, 0, 1) {
			monthStr := strings.ToUpper(current.Format("Jan"))
			label := fmt.Sprintf("%s %d", monthStr, current.Day()) // Ej: "JUN 1"
			periods = append(periods, label)
		}

	case "weekly":
		for current := start; !current.After(end); current = current.AddDate(0, 0, 7) {
			weekEnd := current.AddDate(0, 0, 6)
			startMonth := strings.ToUpper(current.Format("Jan"))
			endMonth := strings.ToUpper(weekEnd.Format("Jan"))

			label := fmt.Sprintf("%s %d - %s %d", startMonth, current.Day(), endMonth, weekEnd.Day()) // Ej: "JUN 1 JUN 7"
			periods = append(periods, label)
		}

	case "monthly":
		current := time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, start.Location())
		endLimit := time.Date(end.Year(), end.Month(), 1, 0, 0, 0, 0, end.Location())

		for !current.After(endLimit) {
			label := strings.ToUpper(current.Format("Jan 2006")) // Ej: "JUN 2023"
			periods = append(periods, label)
			current = current.AddDate(0, 1, 0)
		}
	}

	return periods
}

// filterReadingsByMeter extrae únicamente las lecturas correspondientes a un medidor.
func (uc *ConsumptionUseCase) filterReadingsByMeter(readings []domain.MeterReading, meterID int) []domain.MeterReading {
	var filtered []domain.MeterReading
	for _, r := range readings {
		if r.MeterID == meterID {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

// calculateAggregatedConsumption calcula la diferencia incremental de las lecturas y las acumula en su periodo correspondiente.
func (uc *ConsumptionUseCase) calculateAggregatedConsumption(readings []domain.MeterReading, periods []string, kind string, startDate time.Time) []float64 {
	consumptions := make([]float64, len(periods))
	if len(readings) < 2 {
		return consumptions
	}

	// 1. Ordenamos cronológicamente para garantizar que calculamos diferencias entre horas consecutivas
	sort.Slice(readings, func(i, j int) bool {
		return readings[i].Timestamp.Before(readings[j].Timestamp)
	})

	// 2. Calculamos los deltas (Lectura Actual - Lectura Anterior)
	for i := 1; i < len(readings); i++ {
		delta := readings[i].Value - readings[i-1].Value
		if delta < 0 {
			continue // Evitamos anomalías si un medidor llegó a reiniciarse a 0
		}

		// 3. Identificamos matemáticamente a qué índice del arreglo pertenece el timestamp
		index := uc.getPeriodIndex(readings[i].Timestamp, startDate, kind)

		// 4. Si cae dentro del rango del arreglo, acumulamos la energía consumida
		if index >= 0 && index < len(periods) {
			consumptions[index] += delta
		}
	}

	return consumptions
}

// getPeriodIndex calcula la posición exacta del índice basándose en la distancia temporal desde startDate.
func (uc *ConsumptionUseCase) getPeriodIndex(t time.Time, startDate time.Time, kind string) int {
	tNorm := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	startNorm := time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, startDate.Location())

	switch strings.ToLower(kind) {
	case "daily":
		return int(tNorm.Sub(startNorm).Hours() / 24)
	case "weekly":
		days := int(tNorm.Sub(startNorm).Hours() / 24)
		if days < 0 {
			return -1
		}
		return days / 7
	case "monthly":
		return (tNorm.Year()-startNorm.Year())*12 + int(tNorm.Month()-startNorm.Month())
	default:
		return -1
	}
}
