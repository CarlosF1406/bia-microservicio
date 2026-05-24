# ==============================================================================
# Variables de entorno
# ==============================================================================
ifneq (,$(wildcard ./.env))
    include .env
    export
endif

# ==============================================================================
# Comandos principales
# ==============================================================================
.PHONY: migrate import api test setup run

# Ejecutar las migraciones SQL en el contenedor de la base de datos
migrate:
	@echo "Esperando a que PostgreSQL esté listo para recibir conexiones..."
	@until docker exec bia_postgres_container pg_isready -U $(DB_USER) -d $(DB_NAME) > /dev/null 2>&1; do sleep 1; done
	@echo "Ejecutando migraciones SQL..."
	@docker exec -i bia_postgres_container psql -U $(DB_USER) -d $(DB_NAME) < migrations/init.sql

# Cargar los datos iniciales desde el archivo CSV
import:
	@echo "Importando datos desde el CSV..."
	@go run cmd/importer/main.go

# Iniciar el servidor HTTP de la API
api:
	@echo "Iniciando el servidor HTTP..."
	@go run cmd/api/main.go

# Ejecutar la suite de pruebas unitarias con resumen de cobertura en consola
test:
	@echo "Ejecutando todas las pruebas unitarias del proyecto..."
	go test -v -cover ./...

# Ejecutar pruebas de integración contra la base de datos real en Docker
test-integration:
	@echo "Ejecutando pruebas de integración contra PostgreSQL real..."
	go test -v -tags=integration ./...

# ==============================================================================
# Atajos de flujo de trabajo
# ==============================================================================

# Inicialización completa: Migra la base de datos, importa datos y enciende la API
setup: migrate import api

# Ejecución estándar: Inicia el servidor asumiendo que la base de datos ya está lista
run: api

# Inicialización de pruebas de integración: Migra la base de datos, importa datos y ejecuta las pruebas de integración
integration-setup: migrate import test-integration
