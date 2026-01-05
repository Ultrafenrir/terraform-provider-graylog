###############################
# Kafka Raw Input — full configuration example
#
# Пример показывает:
# - минимальную конфигурацию (bootstrap_servers + topics)
# - расширенную конфигурацию потребителя Kafka (см. раздел Advanced)
# - SSL/SASL настройки (закомментированы; раскомментируйте при необходимости)
#
# Примечания:
# - Набор доступных ключей может отличаться между Graylog 5/6/7 и сборками плагина Kafka.
# - Значения передаются «как есть» в Graylog API через map(dynamic), типы важны:
#   строки (string), числа (int), булевы (bool), списки (list(string)).
###############################

provider "graylog" {
  url   = "http://localhost:9000/api"
  token = "admin-token"
}

resource "graylog_input" "kafka_raw" {
  title  = "kafka-raw"
  type   = "org.graylog2.inputs.raw.kafka.RawKafkaInput"
  global = true

  # --- Minimal configuration ---
  # Достаточно указать брокеры и список топиков (или topic_pattern/topic_filter).
  configuration = {
    # Брокеры Kafka
    bootstrap_servers = ["localhost:9092"]

    # Список топиков для подписки (взаимоисключимо с topic_pattern/topic_filter)
    topics = ["logs"]

    # Минимальный размер выборки сообщений (байты)
    fetch_min_bytes = 1

    # --- Recommended base consumer options ---
    # Идентификатор группы потребителей
    group_id = "graylog-kafka-raw"

    # Поведение при отсутствии оффсета в группе: earliest | latest | none
    auto_offset_reset = "latest"

    # Максимум сообщений за один poll (оптимизация производительности)
    max_poll_records = 500

    # Таймаут запроса (мс)
    request_timeout_ms = 30000

    # Разрешать авто‑создание топиков брокером
    allow_auto_create_topics = false

    # Количество потоков обработки (worker threads) — опционально
    # threads = 1

    # Переопределить source поля у сообщений — опционально
    # override_source = "kafka"

    # --- Fetch/receive tuning ---
    # Максимальный размер данных, получаемый за один fetch с партиции
    # max_partition_fetch_bytes = 1048576
    # Максимальный размер данных за один fetch (совокупно)
    # fetch_max_bytes = 52428800
    # Максимальное ожидание данных в fetch (мс)
    # fetch_max_wait_ms = 500

    # --- Session/heartbeat/poll ---
    # session_timeout_ms = 10000
    # heartbeat_interval_ms = 3000
    # max_poll_interval_ms = 300000

    # --- Retry/backoff ---
    # retries = 3
    # retry_backoff_ms = 100
    # reconnect_backoff_ms = 50

    # --- Network/buffers ---
    # connections_max_idle_ms = 540000
    # receive_buffer_bytes   = 65536
    # send_buffer_bytes      = 131072

    # --- Topic pattern alternative ---
    # topic_pattern = "logs-.*"     # альтернатива topics
    # topic_filter  = "logs-*"      # поддерживается в некоторых версиях

    # --- Security (uncomment to use) ---
    # security_protocol = "SSL"      # PLAINTEXT | SSL | SASL_PLAINTEXT | SASL_SSL

    # SSL options
    # ssl_truststore_location = "/path/to/truststore.jks"
    # ssl_truststore_password = "changeit"
    # ssl_keystore_location   = "/path/to/keystore.jks"
    # ssl_keystore_password   = "changeit"
    # ssl_key_password        = "changeit"

    # SASL options (пример SASL/PLAIN)
    # sasl_mechanism = "PLAIN"
    # sasl_username  = "user"
    # sasl_password  = "pass"
    # Пример SASL JAAS (альтернативно username/password):
    # sasl_jaas_config = "org.apache.kafka.common.security.plain.PlainLoginModule required username=\"user\" password=\"pass\";"

    # Kerberos (если используется)
    # sasl_kerberos_service_name = "kafka"
  }
}
