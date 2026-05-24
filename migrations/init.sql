-- Creación de la tabla de lecturas de consumo
CREATE TABLE IF NOT EXISTS meter_readings (
    id UUID PRIMARY KEY,
    meter_id INT NOT NULL,
    value DOUBLE PRECISION NOT NULL,
    timestamp TIMESTAMPTZ NOT NULL
);

-- ÍNDICE CRÍTICO PARA RENDIMIENTO:
-- Como nuestra API siempre buscará por rangos de fechas y medidores específicos,
-- un índice compuesto acelerará las consultas de milisegundos a microsegundos.
CREATE INDEX IF NOT EXISTS idx_meter_readings_query
ON meter_readings (meter_id, timestamp);
