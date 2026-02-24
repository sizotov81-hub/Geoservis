# Hugoproxy Monitoring

Прокси-сервис для мониторинга и геокодирования адресов с интеграцией Dadata API.

## Содержание

- [Возможности](#возможности)
- [Технологии](#технологии)
- [Быстрый старт](#быстрый-старт)
- [Конфигурация](#конфигурация)
- [Логирование](#логирование)
- [API](#api)
- [Миграции БД](#миграции-бд)
- [Метрики](#метрики)
- [Профилирование](#профилирование)

## Возможности

- ✅ Геокодирование адресов (координаты → адрес)
- ✅ Поиск адресов по строке запроса
- ✅ Кэширование результатов (in-memory)
- ✅ JWT аутентификация
- ✅ Интеграция с Dadata API
- ✅ Prometheus метрики
- ✅ Pprof профилирование
- ✅ Структурированное логирование (zap)
- ✅ RequestID для трассировки запросов
- ✅ Graceful shutdown
- ✅ Security headers (XSS, clickjacking защита)
- ✅ CORS middleware
- ✅ Rate limiting (защита от brute-force)
- ✅ Валидация email и пароля

## Технологии

| Компонент | Технология |
|-----------|------------|
| Язык | Go 1.24+ |
| Фреймворк | Chi Router |
| БД | PostgreSQL |
| Кэш | In-memory |
| Логгер | zap |
| Метрики | Prometheus |
| Миграции | goose |
| Auth | JWT (jwtauth) |
| Security | Security Headers, CORS, Rate Limiting |
| Validation | regexp + custom validators |

## Быстрый старт

### Требования

- Go 1.23+
- Docker и Docker Compose
- PostgreSQL 13+

### Запуск через Docker Compose

```bash
cd hugoproxy-monitoring

# Запуск всех сервисов
docker-compose up -d

# Просмотр логов
docker-compose logs -f hugoproxy

# Остановка
docker-compose down
```

### Локальный запуск

```bash
cd proxy

# Установка зависимостей
go mod download

# Запуск
go run .
```

Сервер запустится на `http://localhost:8080`.

## Конфигурация

Все настройки задаются через переменные окружения или файл `.env`.

### Переменные окружения

| Переменная | Описание | По умолчанию |
|------------|----------|--------------|
| `SERVER_HOST` | Хост сервера | `0.0.0.0` |
| `SERVER_PORT` | Порт сервера | `8080` |
| `SERVER_READ_TIMEOUT` | Таймаут чтения | `10s` |
| `SERVER_WRITE_TIMEOUT` | Таймаут записи | `10s` |
| `SERVER_SHUTDOWN_TIMEOUT` | Таймаут shutdown | `5s` |
| `DB_HOST` | Хост БД | `localhost` |
| `DB_PORT` | Порт БД | `5432` |
| `DB_USER` | Пользователь БД | `postgres` |
| `DB_PASSWORD` | Пароль БД | - |
| `DB_NAME` | Имя БД | `geoservice` |
| `DB_MAX_OPEN_CONNS` | Макс. соединений | `25` |
| `DB_MAX_IDLE_CONNS` | Макс. idle соединений | `25` |
| `DB_CONN_MAX_LIFETIME` | Время жизни соединения | `5m` |
| `MIGRATIONS_PATH` | Путь к миграциям | `migrations` |
| `JWT_SECRET` | Секрет JWT | - |
| `DADATA_API_KEY` | API ключ Dadata | - |
| `DADATA_SECRET_KEY` | Secret ключ Dadata | - |
| `WORKER_ENABLED` | Включить воркер | `false` |
| `WORKER_FILE_PATH` | Путь файла воркера | `/app/static/_index.md` |
| `WORKER_INTERVAL` | Интервал воркера | `1s` |
| `LOG_LEVEL` | Уровень логов | `info` |
| `LOG_FILE_PATH` | Путь к файлу логов | - |
| `CORS_ALLOWED_ORIGINS` | Разрешённые CORS origins | `*` |
| `RATE_LIMIT_RPS` | Rate limit (запросов в секунду) | `10` |
| `RATE_LIMIT_BURST` | Rate limit burst | `20` |

### Пример `.env`

```bash
# Server
SERVER_HOST=0.0.0.0
SERVER_PORT=8080

# Database
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=geoservice

# JWT
JWT_SECRET=your-secret-key

# Dadata
DADATA_API_KEY=your-api-key
DADATA_SECRET_KEY=your-secret-key

# Logging
LOG_LEVEL=info
LOG_FILE_PATH=/var/log/hugoproxy/app.log
```

## Логирование

Сервис использует **zap** для структурированного логирования в формате JSON.

### Уровни логирования

| Уровень | Описание |
|---------|----------|
| `debug` | Отладочная информация (запросы, ответы) |
| `info` | Общая информация (старт/остановка, успешные операции) |
| `warn` | Предупреждения (некритичные ошибки) |
| `error` | Ошибки (неудачные запросы, исключения) |

### Настройка

```bash
# Только stdout (по умолчанию)
LOG_LEVEL=info

# С записью в файл
LOG_LEVEL=info
LOG_FILE_PATH=/var/log/hugoproxy/app.log
```

### Формат логов

```json
{
  "level": "info",
  "timestamp": "2026-02-24T12:00:00Z",
  "msg": "Server starting",
  "addr": ":8080"
}
```

```json
{
  "level": "info",
  "timestamp": "2026-02-24T12:00:01Z",
  "msg": "Search: success",
  "request_id": "550e8400-e29b-41d4-a716-446655440000",
  "query": "Москва Ленина 11",
  "results": 5
}
```

### RequestID

Каждый запрос получает уникальный `request_id`, который передаётся через:
- Заголовок ответа `X-Request-ID`
- Все логи в контексте запроса
- Context для трассировки между сервисами

## API

### Аутентификация

#### Регистрация
```bash
POST /api/register
Content-Type: application/json

{
  "email": "user@example.com",
  "password": "password123"
}
```

#### Вход
```bash
POST /api/login
Content-Type: application/json

{
  "email": "user@example.com",
  "password": "password123"
}

# Ответ:
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

### Геокодирование

#### Поиск адреса
```bash
POST /api/address/search
Authorization: Bearer {token}
Content-Type: application/json

{
  "query": "Москва Ленина 11"
}
```

#### Геокодирование координат
```bash
POST /api/address/geocode
Authorization: Bearer {token}
Content-Type: application/json

{
  "lat": "55.7558",
  "lng": "37.6173"
}
```

### Пользователи

```bash
# Получить список
GET /api/users?limit=10&offset=0
Authorization: Bearer {token}

# Получить по ID
GET /api/users/{id}
Authorization: Bearer {token}

# Получить по email
GET /api/users/email?email=user@example.com
Authorization: Bearer {token}

# Создать
POST /api/users
Authorization: Bearer {token}
Content-Type: application/json

{
  "email": "user@example.com",
  "password": "password123"
}

# Обновить
PUT /api/users/{id}
Authorization: Bearer {token}
Content-Type: application/json

{
  "email": "newemail@example.com"
}

# Удалить
DELETE /api/users/{id}
Authorization: Bearer {token}
```

### Swagger UI

Документация доступна по адресу: `http://localhost:8080/swagger/index.html`

## Миграции БД

Миграции выполняются автоматически при старте сервиса.

### Структура файлов миграций

```
migrations/
└── 20250929190009_create_users_table.sql
```

Формат имени: `YYYYMMDDHHMMSS_description.sql`

### Ручное выполнение миграций

```bash
# Вверх (применить)
goose -dir migrations postgres "postgres://user:pass@localhost/db" up

# Вниз (откатить)
goose -dir migrations postgres "postgres://user:pass@localhost/db" down

# Статус
goose -dir migrations postgres "postgres://user:pass@localhost/db" status
```

## Метрики

Prometheus метрики доступны по адресу: `http://localhost:8080/metrics`

### Доступные метрики

| Метрика | Описание |
|---------|----------|
| `http_requests_total` | Всего HTTP запросов |
| `http_request_duration_seconds` | Длительность HTTP запросов |
| `external_api_requests_total` | Запросы к внешним API |
| `external_api_request_duration_seconds` | Длительность внешних запросов |
| `cache_requests_total` | Запросы к кэшу |
| `cache_hits_total` | Попадания в кэш |

### Grafana

В проекте включена Grafana с автоматическим provisioning:
- URL: `http://localhost:3000`
- Логин: `admin`
- Пароль: `admin`

## Профилирование

Pprof endpoints доступны по адресу `/mycustompath/pprof/`.

### API для профилирования

```bash
# Записать CPU профиль (30 сек)
POST /api/pprof/cpu/start
Authorization: Bearer {token}

# Сделать heap snapshot
POST /api/pprof/heap
Authorization: Bearer {token}

# Записать trace профиль
POST /api/pprof/trace/start
Authorization: Bearer {token}

# Список доступных профилей
GET /api/pprof/profiles
Authorization: Bearer {token}
```

### Стандартные pprof endpoints (через middleware)

```bash
# CPU профиль (через pprof.Handler)
GET /mycustompath/pprof/profile?seconds=30
Authorization: Bearer {token}

# Heap профиль
GET /mycustompath/pprof/heap
Authorization: Bearer {token}

# Trace
GET /mycustompath/pprof/trace
Authorization: Bearer {token}

# Все доступные профили
GET /mycustompath/pprof/
Authorization: Bearer {token}
```

### Просмотр профилей

```bash
# CPU профиль
go tool pprof http://localhost:8080/mycustompath/pprof/profile?seconds=30

# Heap профиль
go tool pprof http://localhost:8080/mycustompath/pprof/heap

# Trace
go tool trace http://localhost:8080/mycustompath/pprof/trace
```

## Безопасность

### Security Headers

Сервис автоматически добавляет заголовки безопасности:

```
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
X-XSS-Protection: 1; mode=block
Content-Security-Policy: default-src 'self'
Referrer-Policy: strict-origin-when-cross-origin
Permissions-Policy: geolocation=(), microphone=(), camera=()
```

### CORS

Настройка кросс-доменных запросов через переменную окружения:

```bash
# Разрешить несколько доменов
CORS_ALLOWED_ORIGINS=https://app.com,https://admin.com

# Или все домены (не рекомендуется для production)
CORS_ALLOWED_ORIGINS=*
```

### Rate Limiting

Защита от brute-force атак:

- **10 запросов в секунду** на IP
- **Burst: 20** запросов

При превышении лимита возвращается статус `429 Too Many Requests`.

### Валидация данных

- **Email**: regex валидация + макс. длина 254 символа
- **Пароль**: мин. 8 символов

### JWT Токены

- Алгоритм: **HS256**
- Требуется переменная `JWT_SECRET` (обязательно!)
- Токен передаётся в заголовке: `Authorization: Bearer {token}`

## Обработка ошибок

Сервис использует обёрнутые ошибки с `fmt.Errorf` и `%w` для сохранения контекста:

```go
// Пример
return fmt.Errorf("failed to create user in repository: %w", err)
```

Для проверки ошибок используйте `errors.Is()`:

```go
if errors.Is(err, service.ErrUserNotFound) {
    // Пользователь не найден
}
```

## Структура проекта (Clean Architecture)

```
hugoproxy-monitoring/
├── proxy/
│   ├── cmd/
│   │   └── server/           # Точка входа (main.go, app.go, shutdown.go)
│   ├── internal/
│   │   ├── domain/           # Domain Layer
│   │   │   ├── entity/       # Модели данных
│   │   │   └── repository/   # Интерфейсы репозиториев
│   │   ├── usecase/          # Use Case Layer (бизнес-логика)
│   │   │   ├── user/         # User use cases
│   │   │   └── geo/          # Geo use cases
│   │   ├── interface/        # Interface Layer
│   │   │   ├── http/
│   │   │   │   ├── handler/  # HTTP handlers (controllers)
│   │   │   │   │   ├── auth/ # Auth handlers
│   │   │   │   │   └── ...
│   │   │   │   └── middleware/ # HTTP middleware
│   │   │   └── persistence/  # Repository implementations
│   │   └── infrastructure/   # Infrastructure Layer
│   │       ├── cache/        # In-memory cache
│   │       ├── db/           # PostgreSQL + migrations
│   │       ├── geo_proxy/    # Geo service proxy
│   │       ├── logger/       # Zap logger
│   │       ├── metrics/      # Prometheus metrics
│   │       ├── middleware/   # RequestID, Security, CORS, RateLimit
│   │       ├── pprof/        # Profiling
│   │       └── worker/       # Background worker
│   ├── pkg/
│   │   └── responder/        # HTTP responder
│   ├── migrations/           # SQL миграции
│   └── docs/                 # Swagger документация
├── grafana/
├── prometheus/
└── docker-compose.yml
```

## Лицензия

Apache 2.0
