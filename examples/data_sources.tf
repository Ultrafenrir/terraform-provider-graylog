###############################
# Data sources examples
###############################

## Provider is configured elsewhere in your root; omitted here for brevity.

# Lookup index set by title (example; adapt to your env)
data "graylog_index_set" "by_title" {
  title = "main-index"
}

# Lookup a stream by title
data "graylog_stream" "by_title" {
  title = "terraform-stream"
}

# Lookup an input by title
data "graylog_input" "by_title" {
  title = "kafka-json"
}

# Lookup a user by username
data "graylog_user" "alice" {
  username = "alice"
}

# Lookup a dashboard by id
data "graylog_dashboard" "main" {
  id = "<dashboard-id>"
}

# Lookup an event notification by id
data "graylog_event_notification" "email" {
  id = "<notification-id>"
}

# New list data sources (V1)
data "graylog_streams" "all" {}
data "graylog_dashboards" "all" {}
data "graylog_index_sets" "all" {}
data "graylog_event_notifications" "all" {}
data "graylog_inputs" "all" {}
data "graylog_users" "all" {}

output "streams_map" {
  value = data.graylog_streams.all.title_map
}

output "dashboards_map" {
  value = data.graylog_dashboards.all.title_map
}

output "index_sets_map" {
  value = data.graylog_index_sets.all.title_map
}

output "notifications_map" {
  value = data.graylog_event_notifications.all.title_map
}

output "inputs_map" {
  value = data.graylog_inputs.all.title_map
}

output "users_map" {
  value = data.graylog_users.all.title_map
}
