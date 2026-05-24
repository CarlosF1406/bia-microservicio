package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"

	"bia-microservicio/internal/adapter/apihttp"
	"bia-microservicio/internal/adapter/client"
	"bia-microservicio/internal/adapter/repository"
	"bia-microservicio/internal/usecase"

	_ "bia-microservicio/docs"

	_ "github.com/lib/pq"
	httpSwagger "github.com/swaggo/http-swagger"
)

// @title           BIA Microservicio de Consumos
// @version         1.0
// @description     API para consultar consumos de energía por medidor
// @host            localhost:8080
// @BasePath        /
func main() {
	log.Println("Iniciando el microservicio de consumos BIA...")

	// 1. Leer variables de entorno con fallbacks seguros
	dbHost := getEnv("DB_HOST", "localhost")
	dbPort := getEnv("DB_PORT", "5432")
	dbUser := getEnv("DB_USER", "postgres")
	dbPass := getEnv("DB_PASSWORD", "postgres")
	dbName := getEnv("DB_NAME", "bia_db")
	apiPort := getEnv("API_PORT", "8080")

	// 2. Conectar a la Base de Datos
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		dbUser, dbPass, dbHost, dbPort, dbName)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Error crítico al abrir la BD: %v", err)
	}
	defer db.Close()

	// Verificar que la base de datos realmente responda
	if err := db.Ping(); err != nil {
		log.Fatalf("No se pudo establecer conexión real con la BD: %v", err)
	}
	log.Println("Conexión a la base de datos establecida con éxito.")

	// 3. Inicializar Adaptadores Secundarios (Infraestructura / Mundo Exterior)
	repo := repository.NewPostgresConsumptionRepository(db)
	addressClient := client.NewHTTPAddressClient()

	// 4. Inicializar Caso de Uso e Inyectar Dependencias (Núcleo del Negocio)
	consumptionUseCase := usecase.NewConsumptionUseCase(repo, addressClient)

	// 5. Inicializar Adaptador Primario (Transporte / HTTP Handler)
	consumptionHandler := apihttp.NewConsumptionHandler(consumptionUseCase)

	// 6. Registrar Rutas en el Multiplexor Nativo de Go
	http.Handle("/consumption", consumptionHandler)
	http.Handle("/swagger/", httpSwagger.WrapHandler)

	// 7. Encender el Servidor Web
	serverAddr := ":" + apiPort
	log.Printf("Servidor HTTP corriendo en el puerto %s (http://localhost%s)", apiPort, serverAddr)
	log.Printf("Documentación Swagger disponible en http://localhost%s/swagger/index.html", serverAddr)

	if err := http.ListenAndServe(serverAddr, nil); err != nil {
		log.Fatalf("Error al encender el servidor HTTP: %v", err)
	}
}

// Función auxiliar para leer variables de entorno
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if len(value) == 0 {
		return defaultValue
	}
	return value
}
