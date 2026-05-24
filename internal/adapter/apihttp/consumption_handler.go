package apihttp

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"bia-microservicio/internal/usecase"
)

// ConsumptionHandler expone los endpoints HTTP para la consulta de consumos.
type ConsumptionHandler struct {
	useCase *usecase.ConsumptionUseCase
}

// NewConsumptionHandler crea una instancia del manejador HTTP.
func NewConsumptionHandler(uc *usecase.ConsumptionUseCase) *ConsumptionHandler {
	return &ConsumptionHandler{
		useCase: uc,
	}
}

// @Summary      Obtener consumos
// @Description  Retorna el consumo acumulado por periodo para los medidores dados
// @Tags         consumption
// @Param        meters_ids   query  string  true  "IDs de medidores separados por coma"
// @Param        start_date   query  string  true  "Fecha inicio (YYYY-MM-DD)"
// @Param        end_date     query  string  true  "Fecha fin (YYYY-MM-DD)"
// @Param        kind_period  query  string  true  "Tipo de periodo: daily, weekly, monthly"
// @Success      200  {object}  domain.ConsumptionResponse
// @Failure      400  {object}  map[string]string
// @Router       /consumption [get]
func (h *ConsumptionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Restringimos el método únicamente a GET
	if r.Method != http.MethodGet {
		h.respondWithError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	queryParams := r.URL.Query()

	// 1. Parsear los IDs de los medidores (?meters_ids=1,2,3)
	metersStr := queryParams.Get("meters_ids")
	if metersStr == "" {
		h.respondWithError(w, http.StatusBadRequest, "missing query parameter: meters_ids")
		return
	}

	var meterIDs []int
	for _, idStr := range strings.Split(metersStr, ",") {
		id, err := strconv.Atoi(strings.TrimSpace(idStr))
		if err != nil {
			h.respondWithError(w, http.StatusBadRequest, "invalid meter_id format, must be integers")
			return
		}
		meterIDs = append(meterIDs, id)
	}

	// 2. Parsear fecha de inicio (?start_date=2023-06-01)
	startDateStr := queryParams.Get("start_date")
	if startDateStr == "" {
		h.respondWithError(w, http.StatusBadRequest, "missing query parameter: start_date")
		return
	}
	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		h.respondWithError(w, http.StatusBadRequest, "invalid start_date format, use YYYY-MM-DD")
		return
	}

	// 3. Parsear fecha de fin (?end_date=2023-07-10)
	endDateStr := queryParams.Get("end_date")
	if endDateStr == "" {
		h.respondWithError(w, http.StatusBadRequest, "missing query parameter: end_date")
		return
	}
	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		h.respondWithError(w, http.StatusBadRequest, "invalid end_date format, use YYYY-MM-DD")
		return
	}

	// Mover el tiempo al último segundo del día final para corregir el bug de la medianoche
	endDate = endDate.Add(23*time.Hour + 59*time.Minute + 59*time.Second)

	// 4. Capturar el tipo de periodo (?kind_period=monthly)
	kindPeriod := queryParams.Get("kind_period")
	if kindPeriod == "" {
		h.respondWithError(w, http.StatusBadRequest, "missing query parameter: kind_period")
		return
	}

	// 5. Ejecutar la lógica de negocio pasando el contexto de la petición HTTP
	response, err := h.useCase.Execute(r.Context(), meterIDs, startDate, endDate, kindPeriod)
	if err != nil {
		// Si el error es por validación de fechas de negocio, devolvemos 400. De lo contrario, 500.
		if strings.Contains(err.Error(), "start_date cannot be after") {
			h.respondWithError(w, http.StatusBadRequest, err.Error())
		} else {
			h.respondWithError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	// 6. Responder de forma exitosa al cliente
	h.respondWithJSON(w, http.StatusOK, response)
}

// Función auxiliar para centralizar las respuestas de error en formato JSON
func (h *ConsumptionHandler) respondWithError(w http.ResponseWriter, code int, message string) {
	h.respondWithJSON(w, code, map[string]string{"error": message})
}

// Función auxiliar para serializar cualquier estructura a JSON
func (h *ConsumptionHandler) respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(payload)
}
