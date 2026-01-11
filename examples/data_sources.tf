###############################
# Data sources examples
###############################

provider "graylog" {
  url   = "http://localhost:9000/api"
  token = "admin-token"
}

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
