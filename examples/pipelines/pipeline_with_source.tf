################################
# Pipeline with source content
################################

provider "graylog" {
  url   = "http://localhost:9000/api"
  token = "admin-token"
}

resource "graylog_pipeline" "sanitize" {
  title       = "sanitize"
  description = "Drop empty messages"
  source = <<-EOT
    pipeline "sanitize"
    stage 0 match either
    rule "drop_empty";

    rule "drop_empty"
    when
      to_string($message.message) == ""
    then
      drop_message();
    end
  EOT
}
