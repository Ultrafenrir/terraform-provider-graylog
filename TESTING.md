# Testing Guide

## Перед каждым релизом

**ВАЖНО**: Всегда запускайте полный набор тестов перед созданием релиза!

```bash
make test-pre-release
```

Эта команда запустит:
1. Unit тесты
2. Integration тесты для Graylog 5.x, 6.x, 7.x
3. Acceptance тесты для Graylog 5.x, 6.x, 7.x
4. Migration тест (обновление state при переходе 5→6→7)

**Примерное время выполнения**: 30-60 минут

## Быстрая проверка

### Unit тесты
```bash
make test-unit
```

### Integration тесты (одна версия Graylog)
```bash
# Для конкретной версии
make GRAYLOG_VERSION=7.0 test-integration

# Для всех версий (5.x, 6.x, 7.x)
make test-integration-all
```

### Acceptance тесты
```bash
# Для конкретной версии
make GRAYLOG_VERSION=7.0 test-acc-one

# Для всех версий
make test-acc-all
```

## Запуск конкретного теста

```bash
# Unit test
make test-unit RUN=TestStreamResource_UpgradeState

# Integration test
make test-integration RUN=TestIndexSet GRAYLOG_VERSION=7.0

# Acceptance test
TF_ACC=1 go test -v -tags=acceptance -run TestAccIndexSet_rotationRetentionConfig ./internal/provider -timeout 30m
```

## Новые тесты добавленные для критических сценариев

### Index Set
- `TestAccIndexSet_rotationRetentionConfig` - проверяет что API не добавляет лишние поля в rotation/retention config

### Stream
- `TestAccStream_disabledFalse` - проверяет что stream с `disabled=false` действительно создаётся включенным
- `TestStreamResource_UpgradeState` - unit test для миграции state v2→v3

## Требования

- Docker и Docker Compose (для integration/acceptance тестов)
- Go 1.24+
- Terraform CLI 1.6+ (для acceptance тестов)
- `jq` (для проверки JSON plan в migration-тесте)

## Что считается успешным live-тестом

- Integration- и acceptance-файлы не могут использовать `httptest` или
  подменённый HTTP transport — это проверяет
  `TestLiveSuitesContainNoHTTPMocks`.
- После apply основные acceptance-тесты читают объект отдельным клиентом
  напрямую из Graylog API; snapshot repositories проверяются через OpenSearch.
- Effect-ресурсы проверяются по фактическому результату: scoped permissions —
  через роль, stream-output binding — через outputs потока.
- Refresh plan после apply обязан быть пустым. `ExpectNonEmptyPlan` в
  acceptance-наборе не используется.

Осознанно не входят в OSS compatibility matrix: legacy
`graylog_dashboard_widget` и `graylog_ldap_setting`. Их endpoints недоступны в
поддерживаемых OSS-образах, поэтому соответствующие тесты явно пропускаются и
не считаются подтверждением покрытия.

Migration-тест строит сохранённый Terraform plan перед каждым apply. Любое
действие с `delete`, в том числе replacement (`delete/create`), немедленно
завершает тест ошибкой. Вывод apply также проверяется на destroy-фазу. После
успешного перехода 5→6→7 удаляется только изолированный Docker-стек с volumes;
`terraform destroy` в migration-тесте не выполняется.

## Переменные окружения

```bash
# Для integration/acceptance тестов
export URL="http://127.0.0.1:9000/api"
export TOKEN=$(printf "admin:admin" | base64)

# Для выбора версии Graylog
export GRAYLOG_VERSION=7.0

# Для выбора версии MongoDB/OpenSearch
export MONGO_TAG=7.0
export OPENSEARCH_TAG=2.17.1
```
