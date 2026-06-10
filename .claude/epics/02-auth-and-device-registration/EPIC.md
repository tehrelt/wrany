# EPIC 02: Auth & Device Registration

## Status

Planned

## Goal

Реализовать фундамент аутентификации пользователей и регистрации устройств.

Бэкенд должен знать, какой пользователь и устройство подключаются, прежде чем разрешить WebSocket-сессии трекера в будущих эпиках.

## Context

EPIC 1 создал заготовки сервисов `tracking-gateway` и `tracking-worker` с Clean Architecture структурой. В `tracking-gateway` уже есть `internal/config`, `internal/transport/http`, `internal/app`. Пока нет ни domain-слоя, ни usecase, ни storage.

Auth-логика размещается в `tracking-gateway`, так как это единственный HTTP-facing сервис. `tracking-worker` в этом эпике не трогается.

## Problem Analysis

1. **Хранение паролей** — нужно bcrypt, plain-text хранить нельзя.
2. **JWT vs сессии** — JWT stateless, но требует механизма отзыва. Refresh tokens в БД дают контроль: можно отозвать конкретный токен без инвалидации всех сессий пользователя.
3. **Утечка email при регистрации** — MVP-компромисс: `register` возвращает `409 Conflict` при дубликате (это приемлемо для UX), но сообщение нейтральное: `unable to register with provided credentials`. Для `login` — всегда одно сообщение: `invalid credentials`, независимо от причины.
4. **Нормализация email** — перед сохранением и поиском: `strings.TrimSpace` + `strings.ToLower`. Unique constraint/index работает по нормализованному значению.
5. **Device identity** — устройство идентифицируется по `(user_id, device_id)`, где `device_id` — UUID, присланный клиентом. Повторная регистрация — upsert, не дубликат.
6. **Миграции** — golang-migrate, SQL-файлы внутри сервиса. Применяются через Makefile.

## Best Practice Research

### Хеширование паролей
- `golang.org/x/crypto/bcrypt` — стандарт для Go. Cost factor 12.
- Не использовать MD5/SHA1/SHA256 для паролей.

### JWT
- `github.com/golang-jwt/jwt/v5` — актуальная версия.
- Access token: TTL 15 мин. Claims — только `sub` (user UUID), `iat`, `exp`. Никаких email, device_id, ролей.
- Refresh token: TTL 7 дней. Хранится в БД как `SHA-256(random 32 bytes)`. Сам токен клиенту отдаётся один раз.
- Секрет JWT — только из env vars, никогда не в коде.
- Refresh token — revoke-модель: `revoked_at TIMESTAMPTZ NULL`. Токен считается валидным если `expires_at > now()` AND `revoked_at IS NULL`.

### Email нормализация
- `strings.TrimSpace(email)` + `strings.ToLower(email)` перед любой операцией.
- Unique constraint/index на нормализованном поле.

### Middleware
- JWT-валидация в HTTP middleware, не в handler.
- `userID` (uuid.UUID) кладётся в `context.Context` через unexported type-safe ключ.
- Handlers извлекают userID через хелпер `auth.UserIDFromContext(ctx)`.

### Миграции
- `github.com/golang-migrate/migrate/v4` — стандарт.
- SQL-файлы: `services/tracking-gateway/infra/migrations/NNNN_description.up.sql` / `.down.sql`.
- Применяются через `make migrate-up` (таргет в корневом Makefile или сервисном).

### Структура ответов API
- Единый envelope: `{"data": ..., "error": null}` / `{"data": null, "error": "message"}`.
- HTTP статус соответствует смыслу (201, 200, 400, 401, 409, 422).

## Solution Design

### Сервис: tracking-gateway

Добавляются слои в существующую Clean Architecture структуру:

```
services/tracking-gateway/
  infra/
    migrations/
      0001_create_users.up.sql
      0001_create_users.down.sql
      0002_create_devices.up.sql
      0002_create_devices.down.sql
      0003_create_refresh_tokens.up.sql
      0003_create_refresh_tokens.down.sql
  internal/
    domain/
      user.go           # User entity, ErrUserNotFound, ErrEmailTaken
      device.go         # Device entity, ErrDeviceNotFound
      token.go          # TokenPair, RefreshToken entity, ErrTokenExpired, ErrTokenRevoked
    usecase/
      auth.go           # AuthUsecase: Register, Login, Refresh + репозиторий-интерфейсы
      device.go         # DeviceUsecase: RegisterDevice, ListDevices
      me.go             # MeUsecase: GetMe
    transport/http/
      auth_handler.go   # POST /v1/auth/register, login, refresh
      device_handler.go # POST /v1/devices/register, GET /v1/devices
      me_handler.go     # GET /v1/me
      middleware.go     # JWT auth middleware + UserIDFromContext helper
      router.go         # обновлённый — добавляет новые маршруты
    storage/postgres/
      user_repo.go      # UserRepository implementation
      device_repo.go    # DeviceRepository implementation
      token_repo.go     # RefreshTokenRepository implementation
```

### Схема таблиц

**users**
```sql
id            UUID        PRIMARY KEY
email         TEXT        NOT NULL UNIQUE   -- нормализованный (trimmed, lowercase)
password_hash TEXT        NOT NULL
created_at    TIMESTAMPTZ NOT NULL
updated_at    TIMESTAMPTZ NOT NULL
```

**devices**
```sql
id           UUID        PRIMARY KEY
user_id      UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE
device_id    UUID        NOT NULL           -- UUID от клиента
name         TEXT        NULL
platform     TEXT        NULL
last_seen_at TIMESTAMPTZ NOT NULL
created_at   TIMESTAMPTZ NOT NULL
updated_at   TIMESTAMPTZ NOT NULL
-- UNIQUE (user_id, device_id)
```

**refresh_tokens**
```sql
id         UUID        PRIMARY KEY
user_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE
token_hash TEXT        NOT NULL UNIQUE
expires_at TIMESTAMPTZ NOT NULL
revoked_at TIMESTAMPTZ NULL               -- NULL = активный, NOT NULL = отозван
created_at TIMESTAMPTZ NOT NULL
```

### Интерфейсы репозиториев (в usecase-пакете)

```go
type UserRepository interface {
    Create(ctx context.Context, user *domain.User) error
    FindByEmail(ctx context.Context, email string) (*domain.User, error)
    FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
}

type DeviceRepository interface {
    Upsert(ctx context.Context, device *domain.Device) error
    ListByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.Device, error)
}

type RefreshTokenRepository interface {
    Save(ctx context.Context, token *domain.RefreshToken) error
    FindByTokenHash(ctx context.Context, hash string) (*domain.RefreshToken, error)
    Revoke(ctx context.Context, id uuid.UUID) error
}
```

### Поток Register
1. Нормализовать email: TrimSpace + ToLower.
2. Валидировать email формат + минимальная длина пароля.
3. bcrypt(password, cost=12).
4. Сохранить User → при нарушении unique constraint → `409`, тело: `{"data":null,"error":"unable to register with provided credentials"}`.
5. Выпустить TokenPair (access JWT + refresh token).
6. Сохранить refresh token (hash) в БД.
7. Вернуть `201` с TokenPair.

### Поток Login
1. Нормализовать email.
2. Найти пользователя по email.
3. bcrypt.CompareHashAndPassword.
4. Если пользователь не найден ИЛИ пароль неверный → `401`, тело: `{"data":null,"error":"invalid credentials"}`. Одно сообщение для обоих случаев.
5. Выпустить TokenPair, сохранить refresh token hash.
6. Вернуть `200` с TokenPair.

### Поток Refresh
1. Принять refresh token из тела.
2. SHA-256(token) → поиск в БД по `token_hash`.
3. Если не найден → `401`.
4. Проверить `expires_at > now()` → если истёк → `401`.
5. Проверить `revoked_at IS NULL` → если отозван → `401`.
6. Revoke старый токен: установить `revoked_at = now()`.
7. Выпустить новый TokenPair, сохранить новый refresh token hash.
8. Вернуть `200` с новым TokenPair.

### Поток Device Register
1. Требует valid JWT (middleware кладёт userID в context).
2. Валидировать `device_id` как UUID.
3. Upsert по `(user_id, device_id)`:
   - Новое устройство → INSERT.
   - Существующее → UPDATE `last_seen_at = now()`, `updated_at = now()`; если в запросе переданы `name`/`platform` — обновить их тоже.
4. Вернуть `201` с device record.

### JWT Claims (access token)
```json
{
  "sub": "<user UUID>",
  "iat": <unix timestamp>,
  "exp": <unix timestamp>
}
```
Никаких email, device_id, ролей или других пользовательских данных в claims.

## Architecture Notes

- Auth-логика только в `tracking-gateway`. `tracking-worker` в этом эпике не трогать.
- Refresh token в БД хранится как `SHA-256(random_bytes)`, сам токен не хранится.
- `revoked_at IS NULL` — единственный признак активности refresh token (вместе с `expires_at`).
- `userID` из JWT передаётся через `context.Context` с unexported type-safe ключом (не строка).
- Все репозитории определяются как интерфейсы в пакете usecase. Реализации — в `internal/storage/postgres/`.
- Handlers отвечают только за HTTP parsing/response: декодировать запрос, вызвать usecase, закодировать ответ.
- Usecase содержит всю бизнес-логику: валидация, нормализация, bcrypt, JWT.
- Storage реализует интерфейсы usecase, содержит только SQL-запросы.
- `main.go` — только composition root: `pgxpool.Connect` → репозитории → usecases → handlers → app.
- Не использовать GORM — только `pgx/v5` напрямую.
- Env vars: `JWT_SECRET`, `JWT_ACCESS_TTL` (default `15m`), `JWT_REFRESH_TTL` (default `168h`), `DATABASE_URL`.

## Migration Strategy

Для миграций используется `golang-migrate/migrate/v4`. Никакого `goose`, `mongoose` или самописного механизма.

Каждый сервис владеет своими миграциями. Файлы миграций лежат внутри директории сервиса:

```
services/<service-name>/infra/migrations/
```

Для EPIC 02 миграции добавляются только в:

```
services/tracking-gateway/infra/migrations/
```

Формат файлов:
```
NNNN_description.up.sql
NNNN_description.down.sql
```

`tracking-worker` в этом эпике не трогать.

### Makefile-команды

```makefile
make migrate-up       # применить все pending миграции
make migrate-down     # откатить последнюю миграцию
make migrate-version  # показать текущую версию миграций
```

Все команды используют `DATABASE_URL` из env.

### Deployment flow (runtime)

Миграции не запускаются на этапе `docker build`. Dockerfile только собирает бинарь.

Запуск миграций — в entrypoint контейнера, **до** старта сервиса:

1. Entrypoint выполняет `migrate -path ... -database $DATABASE_URL up`.
2. Если миграции применились успешно — стартует сервис (`./gateway`).
3. Если migration step завершился ошибкой — контейнер завершается с ненулевым exit code, сервис не стартует.

Для `tracking-gateway` Dockerfile уже существует. В рамках этого эпика нужно обновить `ENTRYPOINT`/`CMD` для поддержки migration step (shell-скрипт или встроенный в бинарь migrate-on-startup).

## Migration Safety Policy

Новые миграции **не могут** приводить к потере данных.

**Запрещено** без отдельного явного согласования:

- `DROP TABLE`
- `DROP COLUMN`
- `TRUNCATE`
- `DELETE` без безопасного `WHERE` и без описанного data migration плана
- `ALTER COLUMN TYPE` при риске потери/искажения данных
- Переименование колонок через `DROP` + `ADD` без backfill
- Удаление индексов/constraints, если это может нарушить целостность данных

Любое потенциально destructive изменение реализуется через **expand/contract**:

1. Добавить новую структуру без удаления старой.
2. Обновить код — работает со старой и новой структурой.
3. Выполнить backfill данных.
4. Переключить чтение/запись на новую структуру.
5. Только в отдельном будущем эпике удалять старые колонки/таблицы после проверки.

Для MVP в EPIC 02 все миграции должны быть **additive/safe**:
- `CREATE TABLE`
- `CREATE INDEX`
- `ADD COLUMN` nullable или с безопасным `DEFAULT`
- `ADD CONSTRAINT` только если существующие данные гарантированно валидны

Down-миграции нужны для локальной разработки. Если down-миграция потенциально удаляет данные — это **должно быть явно отмечено комментарием** в SQL-файле.

## Tasks

- [ ] T01 — Создать SQL-миграции в `services/tracking-gateway/infra/migrations/` (`golang-migrate/migrate/v4`): users, devices, refresh_tokens; down-файлы с комментарием о потере данных
- [ ] T02 — Добавить Makefile-команды: `make migrate-up`, `make migrate-down`, `make migrate-version`; все используют `DATABASE_URL` из env
- [ ] T02a — Обновить `Dockerfile` `tracking-gateway`: добавить shell-entrypoint, который выполняет migration step перед стартом сервиса; если migration step упал — контейнер завершается с ненулевым exit code
- [ ] T03 — Реализовать domain: User, Device, RefreshToken entities и sentinel-ошибки
- [ ] T04 — Реализовать UserRepository (postgres): Create, FindByEmail, FindByID
- [ ] T05 — Реализовать DeviceRepository (postgres): Upsert с обновлением last_seen_at/updated_at/name/platform
- [ ] T06 — Реализовать RefreshTokenRepository (postgres): Save, FindByTokenHash, Revoke
- [ ] T07 — Реализовать AuthUsecase: Register (нормализация email + bcrypt), Login (нейтральный error), Refresh (revoke-модель)
- [ ] T08 — Реализовать DeviceUsecase: RegisterDevice (upsert), ListDevices
- [ ] T09 — Реализовать MeUsecase: GetMe
- [ ] T10 — Реализовать JWT middleware + `UserIDFromContext` helper; claims: sub/iat/exp
- [ ] T11 — Реализовать auth_handler: POST /v1/auth/register, login, refresh
- [ ] T12 — Реализовать device_handler: POST /v1/devices/register, GET /v1/devices
- [ ] T13 — Реализовать me_handler: GET /v1/me
- [ ] T14 — Обновить router.go — добавить новые маршруты с middleware
- [ ] T15 — Обновить config.go — добавить JWT_SECRET, DATABASE_URL, JWT_ACCESS_TTL, JWT_REFRESH_TTL
- [ ] T16 — Обновить app.go — wire всё вместе через pgxpool
- [ ] T17 — Написать unit-тесты usecase (mock-репозитории): register, login, refresh (включая revoked token), device upsert
- [ ] T18 — Написать integration-тесты handlers (testcontainers-postgres)
- [ ] T19 — Обновить `.env.example` новыми переменными
- [ ] T20 — Обновить README `tracking-gateway`: endpoints, env vars, инструкция по миграциям (как запустить локально, deployment flow)

## Acceptance Criteria

- [ ] POST /v1/auth/register → 201, тело содержит `access_token` и `refresh_token`
- [ ] POST /v1/auth/register с дублирующим email → 409, сообщение `unable to register with provided credentials`
- [ ] POST /v1/auth/login → 200, тело содержит `access_token` и `refresh_token`
- [ ] POST /v1/auth/login с неверным паролем → 401, сообщение `invalid credentials`
- [ ] POST /v1/auth/login с несуществующим email → 401, то же сообщение `invalid credentials`
- [ ] POST /v1/auth/refresh → 200, старый refresh token отозван (`revoked_at` установлен), выдан новый
- [ ] POST /v1/auth/refresh с отозванным токеном → 401
- [ ] POST /v1/auth/refresh с просроченным токеном → 401
- [ ] GET /v1/me с валидным JWT → 200, возвращает данные пользователя
- [ ] GET /v1/me без токена / с невалидным → 401
- [ ] POST /v1/devices/register с валидным JWT → 201
- [ ] POST /v1/devices/register повторно с тем же device_id → 200/201, `last_seen_at` обновлён, дублей нет
- [ ] GET /v1/devices → только устройства текущего пользователя
- [ ] Пароли хранятся как bcrypt-хеш, plain-text нигде не фигурирует
- [ ] email нормализуется (lowercase, trimmed) перед сохранением и поиском
- [ ] JWT claims содержат только `sub`, `iat`, `exp`
- [ ] `JWT_SECRET` берётся из env, сервис завершается с ошибкой при старте если переменная не задана
- [ ] Migration tool явно выбран: `golang-migrate/migrate/v4`; никакого `goose` или самописного механизма
- [ ] `make migrate-up` применяет миграции из `services/tracking-gateway/infra/migrations/` без ошибок
- [ ] `make migrate-down` откатывает последнюю миграцию локально
- [ ] `make migrate-version` показывает текущую применённую версию миграций
- [ ] При старте контейнера `tracking-gateway` сначала выполняется migration step, затем стартует сервис
- [ ] Если migration step завершился ошибкой, контейнер завершается с ненулевым exit code; сервис не стартует
- [ ] Все новые миграции EPIC 02 не содержат destructive операций (`DROP TABLE`, `DROP COLUMN`, `TRUNCATE`)
- [ ] `tracking-worker` не получает миграции и не изменяется в рамках EPIC 02
- [ ] `go test ./...` в `services/tracking-gateway` проходит без ошибок

## Test Plan

### Unit-тесты (usecase с mock-репозиториями)
- `AuthUsecase.Register`: успех, дубликат email → нейтральная ошибка, невалидный email, короткий пароль
- `AuthUsecase.Register`: email нормализуется (верхний регистр и пробелы в запросе → нижний регистр в сохранении)
- `AuthUsecase.Login`: успех, неверный пароль → `invalid credentials`, несуществующий email → то же сообщение
- `AuthUsecase.Refresh`: успех с rotate, просроченный токен → 401, отозванный токен → 401, несуществующий → 401
- `DeviceUsecase.RegisterDevice`: первичная регистрация, повторная — upsert обновляет last_seen_at
- `DeviceUsecase.ListDevices`: возвращает только устройства текущего пользователя

### Integration-тесты (handlers с testcontainers-postgres)
- POST /v1/auth/register → 201, тело содержит токены
- POST /v1/auth/register дубль → 409
- POST /v1/auth/login → 200
- POST /v1/auth/login неверный пароль → 401
- POST /v1/auth/refresh → 200, новые токены
- POST /v1/auth/refresh повторно тем же токеном → 401 (revoked)
- GET /v1/me без токена → 401
- GET /v1/me с токеном → 200
- POST /v1/devices/register → 201
- POST /v1/devices/register повторно → upsert, не дубликат
- GET /v1/devices → 200, список содержит зарегистрированное устройство

### Ручная проверка
- `make migrate-up` применяет миграции без ошибок
- Полный flow: register → login → GET /v1/me → refresh → register device → GET /v1/devices через curl/httpie

## Documentation Plan

- Обновить `services/tracking-gateway/README.md`: auth endpoints, env variables, curl-примеры
- Обновить `.env.example`: JWT_SECRET, JWT_ACCESS_TTL, JWT_REFRESH_TTL, DATABASE_URL
- Обновить корневой `README.md`: секция Auth API

## Implementation Log

_Пусто. Заполняется после начала реализации._

## Final Report

_Пусто. Заполняется после завершения эпика._
