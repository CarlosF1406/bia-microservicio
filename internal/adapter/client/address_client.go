package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// AddressResponse define la estructura esperada del JSON del microservicio de direcciones.
type AddressResponse struct {
	MeterID int    `json:"meter_id"`
	Address string `json:"address"`
}

// HTTPAddressClient implementa la interfaz domain.AddressServiceClient
type HTTPAddressClient struct {
	client  *http.Client
	baseURL string
}

// NewHTTPAddressClient crea una nueva instancia del cliente HTTP configurando la URL base.
func NewHTTPAddressClient() *HTTPAddressClient {
	// Leemos la URL del microservicio desde el entorno, con un fallback por defecto
	apiURL := os.Getenv("ADDRESS_SERVICE_URL")
	if apiURL == "" {
		apiURL = "http://localhost:8081" // Supongamos que corre en el puerto 8081
	}

	return &HTTPAddressClient{
		baseURL: apiURL,
		client: &http.Client{
			Timeout: 5 * time.Second, // Blindaje: Ninguna petición externa debe colgar nuestra API
		},
	}
}

// GetAddressByMeterID realiza una petición GET al microservicio externo para obtener la dirección.
func (c *HTTPAddressClient) GetAddressByMeterID(ctx context.Context, meterID int) (string, error) {
	// Construimos la URL del endpoint (ej: http://localhost:8081/addresses/1)
	url := fmt.Sprintf("%s/addresses/%d", c.baseURL, meterID)

	// Creamos la petición HTTP asociándole el contexto (para soportar cancelaciones)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("error al crear la petición de dirección: %w", err)
	}

	// Agregamos headers estándar si fuesen necesarios
	req.Header.Set("Accept", "application/json")

	// Ejecutamos la petición
	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error de red al conectar con el servicio de direcciones: %w", err)
	}
	defer resp.Body.Close()

	// Si el medidor no existe o el servicio falla, devolvemos un error controlado
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("el servicio de direcciones respondió con código: %d", resp.StatusCode)
	}

	// Decodificamos la respuesta JSON
	var addrResp AddressResponse
	if err := json.NewDecoder(resp.Body).Decode(&addrResp); err != nil {
		return "", fmt.Errorf("error al decodificar el JSON de dirección: %w", err)
	}

	return addrResp.Address, nil
}
