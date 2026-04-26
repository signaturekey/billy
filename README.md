# Billy

Billy — учебный платёжный backend на Go. Сервис управляет счетами, переводами и
холдами, хранит денежную историю в PostgreSQL и защищает мутации транзакциями,
блокировками строк и идемпотентными ключами. Redis ускоряет повторную выдачу
завершённых идемпотентных ответов.

Все суммы хранятся в целых минимальных единицах валюты. Авторизация построена на
короткоживущих JWT access-токенах и ротируемых opaque refresh-токенах.

<a id="navigation"></a>

## Навигация

- [Архитектура](#architecture)
- [Структура репозитория](#structure)
- [Быстрый старт](#quick-start)
- [HTTP API](#api)
- [Денежные операции](#money)
- [Авторизация](#authentication)
- [Конфигурация](#configuration)
- [Миграции](#migrations)
- [Тесты и проверки](#testing)
- [Команды разработки](#development)
- [Ограничения](#limitations)

<a id="architecture"></a>

## Архитектура

Billy использует явное разделение HTTP, прикладной логики и хранения данных:

```text
HTTP -> service -> domain
          |
          ├── repository/postgres -> PostgreSQL
          └── cache               -> Redis
```

| Компонент             | Ответственность                                                |
| --------------------- | ------------------------------------------------------------- |
| `transport/http`      | Router, handlers, middleware, DTO и mapping ошибок             |
| `service`             | Прикладные сценарии, проверки прав и денежные инварианты       |
| `domain`              | Сущности и доменные ошибки                                     |
| `cache`               | Best-effort Redis-кэш завершённых идемпотентных ответов        |
| `repository/postgres` | SQL-запросы, транзакции и блокировки                           |
| `worker`              | Фоновое истечение просроченных холдов                          |
| `cmd/billy`           | Сборка зависимостей, запуск HTTP-сервера и graceful shutdown   |

Сервисы зависят от узких repository-интерфейсов, а реализации собираются вручную в
`cmd/billy/main.go`. ORM, DI-контейнер и генерация SQL намеренно не используются:
транзакционные границы и выполняемые запросы остаются явными.

### Стек

| Инструмент         | Роль                                                       |
| ------------------ | ---------------------------------------------------------- |
| Go 1.25            | API-бинарник и фоновый worker                              |
| Gin                | HTTP router и middleware                                   |
| pgx / pgxpool      | PostgreSQL, явные SQL-запросы и транзакции                 |
| Redis + go-redis   | Кэш завершённых идемпотентных ответов                       |
| Goose              | Версионирование SQL-миграций                               |
| cleanenv           | Конфигурация из `.env` и переменных окружения              |
| JWT + bcrypt       | Access-токены и хеширование паролей                        |
| zap                | Структурные логи                                            |
| Testify            | Unit- и integration-тесты                                  |

<a id="structure"></a>

## Структура репозитория

```text
├── cmd/billy/                       точка входа и composition root
├── internal/
│   ├── auth/                        JWT, bcrypt и refresh-токены
│   ├── cache/                       Redis client и idempotency cache
│   ├── config/                      загрузка конфигурации
│   ├── domain/
│   │   ├── entity/                  доменные модели
│   │   └── errors/                  доменные ошибки
│   ├── logger/                      настройка zap
│   ├── pagination/                  разбор page/limit
│   ├── postgres/                    создание пула подключений
│   ├── repository/postgres/         PostgreSQL-репозитории и TxManager
│   ├── service/                     прикладные сценарии и бизнес-правила
│   ├── transport/http/              router, handlers, middleware и DTO
│   └── worker/                      фоновая обработка холдов
├── migrations/                      SQL-миграции Goose
├── .env.example                     пример локальной конфигурации
├── docker-compose.yml               PostgreSQL, Redis и API
├── Dockerfile
└── Makefile
```

Все прикладные пакеты находятся под `internal`: Billy — сервис, а не библиотека, и
не публикует стабильный Go API для внешних модулей.

<a id="quick-start"></a>

## Быстрый старт

### Требования

- Go 1.25.5 или новее;
- Docker с Compose plugin;
- Goose в `PATH` для применения миграций.

Подготовить окружение, PostgreSQL и Redis:

```bash
cp .env.example .env
docker compose up -d db redis
make migrate-up
```

Запустить API в Docker:

```bash
docker compose up -d --build api
```

Или запустить его локально:

```bash
make run
```

Проверить процесс:

```bash
curl http://localhost:8080/health
```

Ожидаемый ответ:

```json
{"status":"ok"}
```

API слушает `APP_PORT`, по умолчанию `8080`. Миграции применяются отдельно: ни
бинарник, ни Docker image не изменяют схему базы данных при старте.

<a id="api"></a>

## HTTP API

Healthcheck находится вне версионированного API. Остальные маршруты используют
префикс `/api/v1`.

| Метод | Маршрут                             | Доступ   | Назначение                     |
| ----- | ----------------------------------- | -------- | ------------------------------ |
| GET   | `/health`                           | публично | Проверка процесса              |
| POST  | `/api/v1/auth/register`             | публично | Регистрация                    |
| POST  | `/api/v1/auth/login`                | публично | Вход                           |
| POST  | `/api/v1/auth/refresh`              | публично | Ротация пары токенов           |
| POST  | `/api/v1/auth/logout`               | auth     | Отзыв refresh-токена           |
| POST  | `/api/v1/accounts`                  | auth     | Создание счёта                 |
| GET   | `/api/v1/accounts/{id}`             | auth     | Получение счёта                |
| GET   | `/api/v1/accounts/{id}/balance`     | auth     | Баланс и доступная сумма       |
| GET   | `/api/v1/accounts/{id}/operations`  | auth     | История операций               |
| POST  | `/api/v1/accounts/{id}/topups`      | auth     | Пополнение                     |
| POST  | `/api/v1/accounts/{id}/withdrawals` | auth     | Списание                       |
| POST  | `/api/v1/transfers`                 | auth     | Перевод между счетами          |
| POST  | `/api/v1/holds`                     | auth     | Создание холда                 |
| POST  | `/api/v1/holds/{id}/confirm`        | auth     | Подтверждение холда            |
| POST  | `/api/v1/holds/{id}/cancel`         | auth     | Отмена холда                   |

Защищённые маршруты ожидают заголовок:

```http
Authorization: Bearer <access_token>
```

Пополнение, списание, перевод и все мутации холдов также требуют:

```http
Idempotency-Key: <unique-key>
```

История операций поддерживает `page` и `limit`. Значения по умолчанию — `1` и `20`,
максимальный `limit` — `100`.

<a id="money"></a>

## Денежные операции

### Инварианты

- суммы хранятся в `BIGINT`, без `float`;
- сумма операции должна быть положительной;
- `balance` и `reserved_amount` не могут быть отрицательными;
- `reserved_amount` не может превышать `balance`;
- доступная сумма равна `balance - reserved_amount`;
- перевод между разными валютами и перевод на тот же счёт запрещены;
- владелец счёта определяется по access-токену, а не по телу запроса.

PostgreSQL дублирует критичные проверки через `CHECK` constraints. Денежная мутация
и запись в ledger коммитятся в одной транзакции.

### Транзакции и блокировки

Списание и изменение резерва читают счёт через `SELECT ... FOR UPDATE`. Перевод
блокирует оба счёта по возрастанию `account_id`, чтобы встречные переводы брали
блокировки в одинаковом порядке.

Ledger сохраняет `balance_before` и `balance_after`. Изменение баланса и запись
истории либо проходят вместе, либо вместе откатываются.

### Идемпотентность

Ключ идентифицируется комбинацией пользователя, типа операции и значения
`Idempotency-Key`. Для него сохраняются хеш запроса, статус и готовый HTTP-ответ.

| Повторный запрос                           | Результат                        |
| ----------------------------------------- | -------------------------------- |
| Тот же ключ и payload, операция завершена | Возвращается сохранённый ответ   |
| Тот же ключ, другой payload               | Конфликт ключа                   |
| Операция с этим ключом ещё выполняется    | Конфликт незавершённой обработки |

Запись ключа и сама денежная мутация выполняются в одной PostgreSQL-транзакции.
После её коммита готовый ответ best-effort сохраняется в Redis на 24 часа. Cache hit
не открывает транзакцию; при cache miss, повреждённом значении или недоступности
Redis сервис прозрачно использует PostgreSQL как источник истины.

### Холды

Создание холда увеличивает `reserved_amount`. Подтверждение уменьшает баланс и
резерв, отмена снимает только резерв. Фоновый worker раз в 10 секунд обрабатывает до
100 просроченных `pending`-холдов и переводит их в `expired`.

Каждый переход повторно проверяет статус под блокировкой, поэтому подтверждение и
автоматическое истечение не могут успешно обработать один холд дважды.

<a id="authentication"></a>

## Авторизация

Регистрация и вход возвращают пару токенов:

- access-токен — JWT HS256 с `sub`, `iat` и `exp`, по умолчанию живёт 15 минут;
- refresh-токен — случайная opaque-строка, по умолчанию живёт 30 дней.

В базе хранится только SHA-256 хеш refresh-токена. При обновлении старый токен
отзывается, а новая пара создаётся в той же транзакции. Logout отзывает переданный
refresh-токен.

<a id="configuration"></a>

## Конфигурация

Конфигурация читается из `.env`, а если файла нет — из окружения. Полный пример
находится в `.env.example`.

| Переменная               | Значение по умолчанию   | Назначение                    |
| ------------------------ | ----------------------- | ----------------------------- |
| `APP_ENV`                | `development`           | Режим логирования             |
| `APP_PORT`               | `8080`                  | Порт HTTP-сервера             |
| `APP_BASE_URL`           | `http://localhost:8080` | Базовый URL приложения        |
| `HOLD_TTL`               | `15m`                   | Время жизни холда             |
| `JWT_SECRET`             | обязательное            | Ключ подписи access-токенов   |
| `ACCESS_TOKEN_TTL`       | `15m`                   | Время жизни access-токена     |
| `REFRESH_TOKEN_TTL`      | `720h`                  | Время жизни refresh-токена    |
| `REDIS_URL`              | `redis://localhost:6379/0` | Подключение к Redis          |
| `REDIS_PORT`             | `6379`                  | Порт Redis, опубликованный Compose |
| `REDIS_DIAL_TIMEOUT`     | `500ms`                 | Таймаут подключения к Redis   |
| `REDIS_READ_TIMEOUT`     | `500ms`                 | Таймаут чтения из Redis       |
| `REDIS_WRITE_TIMEOUT`    | `500ms`                 | Таймаут записи в Redis        |
| `DB_HOST`                | `localhost`             | Хост PostgreSQL               |
| `DB_PORT`                | `5432`                  | Порт PostgreSQL               |
| `DB_USER`                | `postgres`              | Пользователь PostgreSQL       |
| `DB_PASSWORD`            | `postgres`              | Пароль PostgreSQL             |
| `DB_NAME`                | `billy_db`              | База данных                   |
| `DB_SSL_MODE`            | `disable`               | Режим TLS                     |
| `DB_MAX_CONNS`           | `25`                    | Максимум соединений пула      |
| `DB_MIN_CONNS`           | `10`                    | Минимум соединений пула       |
| `DB_MAX_CONN_LIFETIME`   | `5m`                    | Максимальная жизнь соединения |
| `DB_MAX_CONN_IDLE_TIME`  | `30m`                   | Максимальный idle соединения  |
| `DB_HEALTH_CHECK_PERIOD` | `1m`                    | Период проверки пула          |

Значение `JWT_SECRET=change-me` из примера подходит только для локальной разработки.

<a id="migrations"></a>

## Миграции

SQL-миграции лежат в `migrations/` и применяются внешним Goose CLI:

```bash
make migrate-up
make migrate-down
make migrate-status
make migrate-redo
make migrate-reset
make migrate-create name=add_some_table
```

Команды строят DSN из `DB_*` переменных текущего `.env`.

<a id="testing"></a>

## Тесты и проверки

```bash
make test
make test-verbose
make vet
make lint
```

`make test` запускает `go test ./... -race`, сохраняет coverage profile и печатает
покрытие по функциям. Unit-тесты проверяют сервисы и HTTP transport; Redis adapter
тестируется на встроенном `miniredis`, поэтому внешний Redis для тестов не нужен.

Repository integration-тесты требуют отдельный PostgreSQL DSN:

```bash
TEST_POSTGRES_DSN='postgres://postgres:postgres@localhost:5432/billy_test?sslmode=disable' \
  go test ./internal/repository/postgres/...
```

Без `TEST_POSTGRES_DSN` эти тесты пропускаются. Они создают изолированную схему и
применяют SQL из `migrations/`, не используя рабочие таблицы указанной базы.

Конфигурация golangci-lint находится в `.golangci.yml`.

<a id="development"></a>

## Команды разработки

| Команда             | Действие                                   |
| ------------------- | ------------------------------------------ |
| `make build`        | Собрать `bin/billy`                        |
| `make run`          | Запустить API локально                     |
| `make test`         | Запустить тесты с race detector и coverage |
| `make lint`         | Запустить golangci-lint                    |
| `make tidy`         | Выполнить `go mod tidy` и `go mod verify`  |
| `make docker-up`    | Запустить Compose stack                    |
| `make docker-down`  | Остановить Compose stack                   |
| `make docker-build` | Пересобрать Compose images                 |

<a id="limitations"></a>

## Ограничения

Billy сфокусирован на локальном backend-фундаменте. В проекте нет интеграции с
платёжным провайдером, ролей и admin API, message broker, OpenAPI, метрик
Prometheus, readiness endpoint и production deployment-конфигурации.
