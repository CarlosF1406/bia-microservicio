package domain

import "context"

// Address representa la ubicación física asignada a un medidor.
type Address struct {
	MeterID int    `json:"meter_id"`
	Value   string `json:"address"` // Aquí se guardará el string (ej: "Dirección mock")
}

// AddressServiceClient define el contrato para comunicarse con el sistema externo.
// No importa si mañana cambia de REST a GraphQL o gRPC, el caso de uso solo llamará a este método.
type AddressServiceClient interface {
	GetAddressByMeterID(ctx context.Context, meterID int) (string, error)
}
