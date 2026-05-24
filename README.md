# BIA Microservicio de Consumos

Microservicio REST en Go para consultar el consumo de energía de medidores por periodos diarios, semanales y mensuales.

# 📋 Prerrequisitos

Antes de comenzar, asegúrate de tener instaladas las siguientes herramientas en tu sistema:

- **Go (Versión 1.20 o superior):** El lenguaje de programación y entorno de ejecución utilizado para construir el microservicio.
    
- **Docker y Docker Compose:** Necesarios para levantar el contenedor aislado de la base de datos PostgreSQL sin necesidad de instalarla localmente en tu sistema operativo.
    
- **Git:** Para clonar el repositorio y gestionar las ramas del proyecto.
    
- **Cliente HTTP (Postman, Insomnia o `curl` en terminal):** Para realizar las peticiones de prueba al endpoint de la API.

# ⚙️ Variables de Entorno (`.env`)

El proyecto utiliza un archivo `.env` en la raíz para centralizar la configuración. Copia el archivo de ejemplo para crear tu entorno local:

```shell
cp .env.example .env
```

A continuación se detalla el propósito de cada variable:

| **Variable**              | **Valor por Defecto**   | **Descripción**                                                      |
| ------------------------- | ----------------------- | -------------------------------------------------------------------- |
| **`DB_HOST`**             | `localhost`             | Dirección de red donde escucha la base de datos (contenedor Docker). |
| **`DB_PORT`**             | `5432`                  | Puerto expuesto de PostgreSQL.                                       |
| **`DB_USER`**             | `postgres`              | Usuario administrador de la base de datos.                           |
| **`DB_PASSWORD`**         | `postgres`              | Contraseña de acceso para el usuario administrador.                  |
| **`DB_NAME`**             | `bia_db`                | Nombre de la base de datos donde se almacenan las lecturas.          |
| **`ADDRESS_SERVICE_URL`** | `http://localhost:8081` | URL base del microservicio externo de direcciones (Consultas HTTP).  |
| **`API_PORT`**            | `8080`                  | Puerto local en el cual correrá nuestro servidor web de consumos.    |
| **`CSV_FILE_PATH`**       | `data/test_bia.csv`     | Ruta relativa del archivo CSV con los datos históricos de consumo.   |

# 🧠 Asunciones del Negocio

📌 **Nota sobre el Dataset:** Dado que el dataset provisto (`test_bia (1).csv`) contiene una única columna de lectura numérica, el sistema asume dicha variable estrictamente como **energía activa (`active`)**. Para cumplir con la estructura del contrato JSON solicitado en los requerimientos, las energías reactivas y exportadas se inicializan automáticamente en `0`.

📌 **Nota del microservicio de direcciones:** El sistema consulta de manera externa las ubicaciones mediante peticiones HTTP. El caso de uso está diseñado bajo un principio de tolerancia a fallos: **si el servicio externo de direcciones no está disponible, está apagado o el medidor no existe en su registro, la API no fallará**. En su lugar, retornará el texto informativo `"Dirección no disponible"` en el campo de dirección, permitiendo al usuario visualizar los datos del consumo gráfico sin interrupciones.

# 🚀 Instalación y Ejecución del Proyecto

Sigue estos pasos en orden cronológico desde tu terminal para desplegar el microservicio de extremo a extremo:

## 1. Clonar el repositorio y preparar el entorno

```shell
git clone <URL_DE_TU_REPOSITORIO>
cd bia-microservicio
cp .env.example .env
```

## 2. Levantar la Infraestructura (Base de Datos)

```shell
docker compose up -d
```

(Puedes verificar que esté arriba corriendo `docker ps`).

## 3. Insertar los insumos en la carpeta de data

Dentro de la carpeta raíz se encuentra un directorio llamado `data/`. Debes asegurarte de que el archivo CSV de datos históricos esté guardado allí. El nombre del archivo debe coincidir exactamente con el valor declarado en la variable `CSV_FILE_PATH` de tu `.env`.

## 4. Inicializar, importar datos y encender la API (Todo en uno)

```shell
make setup
```

(En caso de que ya se haya inicializado el setup y solo se quiera encender o apagar la API usar `make run`)

# 🧪 Ejecución de Pruebas (Testing)

El proyecto cuenta con una suite completa de pruebas automatizadas dividida en dos capas: **Pruebas Unitarias** (aisladas de la infraestructura) y **Pruebas de Integración** (contra la base de datos real). Ambas se pueden ejecutar de forma cómoda utilizando el `Makefile`.

## 1. Pruebas Unitarias

Estas pruebas validan la lógica de negocio pura de los casos de uso y el comportamiento de los controladores (handlers) utilizando dobles de prueba (*mocks*) en memoria para los repositorios y clientes HTTP. No requieren infraestructura externa activa.

* **Comando para ejecutar todas las pruebas unitarias:**

```shell
make test
```

(**¿Qué hace por detrás?** Corre de forma nativa `go test ./...` omitiendo cualquier lógica que dependa de componentes pesados o conexiones de red.)

## 2. Pruebas de integración

Estas pruebas comprueban la persistencia real de los datos, las consultas complejas y la interacción directa de los adaptadores HTTP con la base de datos PostgreSQL.

⚠️ **Requisito Crítico:** El contenedor de Docker con la base de datos debe estar encendido antes de lanzar estos tests, de lo contrario la conexión fallará.

* **Comando para ejecutar las pruebas de integración:**

```shell
make integration-setup
```

En este caso `make integration-setup` se utiliza para preparar todo el setup (migrar, importar y ejecutar el test) para realizar las pruebas, en caso de que el setup ya este listo, solamente ejecutar `make test-integration`.
