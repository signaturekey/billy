# Billy

Billy - учебный Go backend-сервис для аккаунтов, балансов и денежных операций.

Проект не имитирует настоящий банк и не претендует на production fintech. Его цель - показать базовые backend-навыки на понятной доменной модели: слои приложения, PostgreSQL, транзакции, блокировки строк, идемпотентность, историю операций, background worker, тесты и простую наблюдаемость.

## Какую задачу решает проект

Billy хранит аккаунты пользователей и позволяет выполнять операции с балансом:

- создавать аккаунты в валюте;
- пополнять баланс;
- списывать доступные средства;
- переводить деньги между аккаунтами;
- резервировать средства через holds;
- подтверждать, отменять и автоматически протухать holds;
- смотреть историю операций по аккаунту.

Домен специально небольшой, чтобы сфокусироваться не на количестве функций, а на корректности денежных изменений и границах ответственности между слоями.

## Возможности

- HTTP API на Gin.
- PostgreSQL как основное хранилище.
- Работа с БД через `pgx/v5`, без ORM и без `sqlc`.
- Транзакции для всех денежных мутаций.
- `SELECT ... FOR UPDATE` для блокировки аккаунтов при изменении баланса.
- Ledger-таблица для истории операций.
- Идемпотентность денежных мутаций через `Idempotency-Key`.
- Holds с жизненным циклом `pending -> confirmed | cancelled | expired`.
- Background worker для истечения просроченных holds.
- Структурные логи через `zap`.
- Unit-тесты сервисного слоя, HTTP-контрактов и интеграционные тесты PostgreSQL-репозиториев.

## Стек

- Go
- Gin
- PostgreSQL
- `pgx/v5`
- Docker Compose
- Goose migrations
- Zap logger
- Testify

Redis в текущей версии проекта не используется.

## Архитектура

Проект сделан как модульный слоистый монолит:

```text
cmd/api                         composition root и запуск HTTP-сервера
internal/config                 загрузка конфигурации
internal/domain/entity          доменные сущности и статусы
internal/domain/errors          доменные ошибки
internal/transport/http         router, handlers, DTO, middleware, HTTP responses
internal/service                use cases и бизнес-правила
internal/repository/postgres    PostgreSQL-репозитории
internal/worker                 фоновые задачи
internal/pkg/postgres           подключение к PostgreSQL
internal/pkg/logger             настройка логгера
internal/pkg/pagination         параметры пагинации
migrations                      SQL-миграции
```

`cmd/api` собирает зависимости вручную: создает подключение к БД, репозитории, сервисы, handlers, router и worker. Тяжелые framework-подходы, ORM и генерация SQL-кода здесь намеренно не используются, чтобы было видно, где проходят транзакционные границы и какие SQL-запросы выполняются.

## Доменная модель

Основные сущности:

- `accounts` - аккаунт пользователя в валюте, хранит `balance`, `reserved_amount` и статус.
- `ledger_entries` - история операций по аккаунту с балансом до и после операции.
- `transfers` - переводы между двумя аккаунтами.
- `holds` - резервирование средств с временем истечения.
- `idempotency_keys` - сохраненные результаты денежных мутаций для безопасных повторов.

Типы ledger-записей:

- `topup`
- `withdrawal`
- `transfer_in`
- `transfer_out`
- `hold_created`
- `hold_confirmed`
- `hold_cancelled`
- `hold_expired`

## Денежные инварианты

В проекте деньги хранятся в integer minor units: например, `1000` означает 1000 минимальных единиц валюты. `float` не используется.

Основные правила:

- баланс аккаунта не может быть отрицательным;
- зарезервированная сумма не может быть отрицательной;
- `reserved_amount` не может быть больше `balance`;
- доступный баланс считается как `balance - reserved_amount`;
- списание и перевод проверяют именно доступный баланс;
- сумма операции должна быть положительной;
- валюта аккаунта нормализуется к трехбуквенному uppercase-коду;
- перевод между аккаунтами с разной валютой запрещен;
- перевод на тот же аккаунт запрещен.

Часть инвариантов проверяется в сервисном слое, часть дополнительно закреплена CHECK constraints в PostgreSQL.

## API

Ниже перечислены основные ручки проекта. Все `/api/v1/*` ручки требуют заголовок:

```http
X-User-ID: 1
```

Это упрощенная учебная авторизация: пользователь берется из заголовка, полноценной auth-системы в проекте нет.

### Accounts

| Method | Path | Назначение |
| --- | --- | --- |
| `POST` | `/api/v1/accounts` | Создать аккаунт |
| `GET` | `/api/v1/accounts/:id` | Получить аккаунт |
| `GET` | `/api/v1/accounts/:id/balance` | Получить баланс аккаунта |

### Money operations

| Method | Path | Назначение |
| --- | --- | --- |
| `POST` | `/api/v1/accounts/:id/topups` | Пополнить аккаунт |
| `POST` | `/api/v1/accounts/:id/withdrawals` | Списать средства |

### Transfers

| Method | Path | Назначение |
| --- | --- | --- |
| `POST` | `/api/v1/transfers` | Перевести средства между аккаунтами |

### Holds

| Method | Path | Назначение |
| --- | --- | --- |
| `POST` | `/api/v1/holds` | Создать hold и зарезервировать средства |
| `POST` | `/api/v1/holds/:id/confirm` | Подтвердить hold и списать средства |
| `POST` | `/api/v1/holds/:id/cancel` | Отменить hold и снять резерв |

### Operations history

| Method | Path | Назначение |
| --- | --- | --- |
| `GET` | `/api/v1/accounts/:id/operations?page=1&limit=20` | Получить историю операций |

Пагинация использует `page` и `limit`. По умолчанию `page=1`, `limit=20`, максимальный `limit=100`.

### Health

| Method | Path | Назначение |
| --- | --- | --- |
| `GET` | `/health` | Простая health-проверка сервиса |

Отдельные `/ready` и `/metrics` эндпоинты в текущей версии не реализованы.

## Идемпотентность

Денежные мутации требуют заголовок:

```http
Idempotency-Key: some-unique-key
```

Идемпотентность применяется к:

- topup;
- withdrawal;
- transfer;
- create hold;
- confirm hold;
- cancel hold.

Для ключа сохраняются `user_id`, тип операции, hash запроса, статус обработки и готовый HTTP-ответ. Если тот же пользователь повторяет тот же запрос с тем же ключом, сервис возвращает сохраненный ответ и не выполняет денежную мутацию повторно.

Если тот же ключ используется для другого payload в рамках того же типа операции, сервис возвращает конфликт. Если запрос с таким ключом еще обрабатывается, сервис тоже возвращает конфликт.

## Транзакции и блокировки

Все операции, меняющие деньги, выполняются внутри PostgreSQL-транзакции.

Для изменения аккаунта репозиторий получает строку через `SELECT ... FOR UPDATE`. Это защищает баланс и reserved amount от гонок при параллельных запросах.

Перевод блокирует оба аккаунта в стабильном порядке по `account_id`. Это снижает риск deadlock при встречных переводах, когда два запроса пытаются заблокировать одни и те же аккаунты в разном порядке.

Ledger-записи пишутся в той же транзакции, что и изменение баланса. Поэтому история операций не расходится с фактическим состоянием аккаунта.

## Жизненный цикл hold

Hold резервирует часть доступного баланса:

```text
available = balance - reserved_amount
```

Создание hold:

- проверяет владельца аккаунта и активный статус;
- проверяет доступный баланс;
- увеличивает `reserved_amount`;
- создает запись `holds` со статусом `pending`;
- пишет ledger-запись `hold_created`.

Подтверждение hold:

- возможно только для `pending`;
- запрещено после `expires_at`;
- уменьшает `balance`;
- уменьшает `reserved_amount`;
- переводит hold в `confirmed`;
- пишет `hold_confirmed`.

Отмена hold:

- возможна только для `pending`;
- уменьшает `reserved_amount`;
- переводит hold в `cancelled`;
- пишет `hold_cancelled`.

Истечение hold:

- background worker периодически ищет просроченные `pending` holds;
- снимает резерв;
- переводит hold в `expired`;
- пишет `hold_expired`.

## Наблюдаемость

В проекте есть базовая наблюдаемость:

- структурные логи через `zap`;
- middleware для request id;
- logging middleware для HTTP-запросов;
- recovery middleware для обработки panic;
- `/health` для простой проверки живости процесса.

Метрики Prometheus и readiness-check пока не добавлены.

## Тесты

Тесты покрывают основные уровни:

- сервисный слой: бизнес-правила аккаунтов, списаний, переводов, holds и идемпотентности;
- HTTP-слой: auth-контракт через `X-User-ID`, валидация, маппинг доменных ошибок в HTTP-ответы;
- PostgreSQL-репозитории: интеграционные проверки SQL, constraints, транзакций и чтения/записи.

Важные проверяемые сценарии:

- topup;
- withdrawal;
- transfer;
- holds;
- idempotency replay и конфликты ключей;
- auth/error mapping.

Проект не заявляет 100% coverage. Интеграционные тесты PostgreSQL запускаются только при наличии `TEST_POSTGRES_DSN`; без него они пропускаются.

## Локальный запуск

1. Скопировать переменные окружения:

```bash
cp .env.example .env
```

2. Поднять инфраструктуру:

```bash
docker compose up -d
```

или через Makefile:

```bash
make docker-up
```

3. Применить миграции:

```bash
make migrate-up
```

Команда использует `goose`, поэтому он должен быть установлен локально.

4. Запустить API:

```bash
go run ./cmd/api
```

или:

```bash
make run
```

5. Запустить тесты:

```bash
go test ./...
```

или:

```bash
make test
```

## Переменные окружения

Основные переменные берутся из `.env` или окружения:

| Переменная | Назначение | Значение по умолчанию |
| --- | --- | --- |
| `APP_NAME` | Имя приложения для сборки через Makefile | `billy` в `.env.example` |
| `APP_ENV` | Окружение приложения | `development` |
| `APP_PORT` | HTTP-порт | `8080` |
| `APP_BASE_URL` | Базовый URL приложения | `http://localhost:8080` |
| `HOLD_TTL` | Время жизни hold | `15m` |
| `DB_HOST` | Хост PostgreSQL | `localhost` |
| `DB_PORT` | Порт PostgreSQL | `5432` |
| `DB_USER` | Пользователь PostgreSQL | `postgres` |
| `DB_PASSWORD` | Пароль PostgreSQL | `postgres` |
| `DB_NAME` | Имя БД | `billy_db` |
| `DB_SSL_MODE` | SSL mode для PostgreSQL | `disable` |
| `DB_MAX_CONNS` | Максимум соединений в pool | `25` |
| `DB_MIN_CONNS` | Минимум соединений в pool | `10` |
| `DB_MAX_CONN_LIFETIME` | Max lifetime соединения | `5m` |
| `DB_MAX_CONN_IDLE_TIME` | Max idle time соединения | `30m` |
| `DB_HEALTH_CHECK_PERIOD` | Период health-check pool | `1m` |

## Миграции

Миграции лежат в `migrations/` и написаны в формате Goose.

Полезные команды:

```bash
make migrate-up
make migrate-down
make migrate-status
make migrate-redo
make migrate-reset
```

Создание новой миграции:

```bash
make migrate-create name=add_some_table
```

## Примеры запросов

Во всех примерах предполагается, что API запущен на `http://localhost:8080`.

### Создать аккаунт

```bash
curl -X POST http://localhost:8080/api/v1/accounts \
  -H "Content-Type: application/json" \
  -H "X-User-ID: 1" \
  -d '{"currency":"USD"}'
```

### Пополнить аккаунт

```bash
curl -X POST http://localhost:8080/api/v1/accounts/1/topups \
  -H "Content-Type: application/json" \
  -H "X-User-ID: 1" \
  -H "Idempotency-Key: topup-1" \
  -d '{"amount":10000}'
```

### Списать средства

```bash
curl -X POST http://localhost:8080/api/v1/accounts/1/withdrawals \
  -H "Content-Type: application/json" \
  -H "X-User-ID: 1" \
  -H "Idempotency-Key: withdrawal-1" \
  -d '{"amount":2500}'
```

### Перевести средства

```bash
curl -X POST http://localhost:8080/api/v1/transfers \
  -H "Content-Type: application/json" \
  -H "X-User-ID: 1" \
  -H "Idempotency-Key: transfer-1" \
  -d '{"from_account_id":1,"to_account_id":2,"amount":3000}'
```

### Создать hold

```bash
curl -X POST http://localhost:8080/api/v1/holds \
  -H "Content-Type: application/json" \
  -H "X-User-ID: 1" \
  -H "Idempotency-Key: hold-create-1" \
  -d '{"account_id":1,"amount":1500}'
```

### Подтвердить hold

```bash
curl -X POST http://localhost:8080/api/v1/holds/1/confirm \
  -H "X-User-ID: 1" \
  -H "Idempotency-Key: hold-confirm-1"
```

### Получить историю операций

```bash
curl "http://localhost:8080/api/v1/accounts/1/operations?page=1&limit=20" \
  -H "X-User-ID: 1"
```

## Структура проекта

```text
.
+-- cmd/
|   +-- api/
|       +-- main.go
+-- internal/
|   +-- config/
|   +-- domain/
|   |   +-- entity/
|   |   +-- errors/
|   +-- pkg/
|   |   +-- logger/
|   |   +-- pagination/
|   |   +-- postgres/
|   +-- repository/
|   |   +-- postgres/
|   +-- service/
|   +-- transport/
|   |   +-- http/
|   +-- worker/
+-- migrations/
+-- docker-compose.yml
+-- Dockerfile
+-- Makefile
+-- go.mod
+-- go.sum
```

## Что намеренно не реализовано

- Полноценная регистрация, login, JWT/session-based auth.
- Роли пользователей и admin API.
- Интеграция с платежными провайдерами.
- Реальные банковские, юридические или compliance-процессы.
- Распределенные транзакции.
- Redis/cache layer.
- Message broker.
- Prometheus metrics endpoint.
- Readiness endpoint.
- OpenAPI/Swagger спецификация.
- Production deployment.

Эти вещи не добавлены специально: проект сфокусирован на backend-фундаменте, а не на имитации большой fintech-системы.

## Возможные улучшения

- Добавить OpenAPI-спецификацию.
- Добавить `/ready` с проверкой подключения к PostgreSQL.
- Добавить `/metrics` и базовые Prometheus-метрики.
- Добавить нормальную auth-систему вместо `X-User-ID`.
- Добавить outbox-паттерн для событий по денежным операциям.
- Добавить более подробный audit trail для административных действий.
- Добавить CI pipeline с линтерами, unit и integration tests.
- Улучшить документацию по ошибкам API.
