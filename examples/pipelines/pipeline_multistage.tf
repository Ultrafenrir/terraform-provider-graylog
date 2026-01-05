############################################################
# Example: Multi-stage Pipeline
############################################################

provider "graylog" {
  url   = "http://localhost:9000/api"
  token = "admin-token"
}

resource "graylog_pipeline" "multistage" {
  title       = "multistage"
  description = "Multi-stage sample pipeline"
  source = <<-EOT
    pipeline "multistage"

    stage 0 match either
      rule "normalize_level";

    stage 1 match all
      rule "drop_debug";

    rule "normalize_level"
    when
      has_field("level")
    then
      set_field("level", to_uppercase(to_string($message.level)));
    end

    rule "drop_debug"
    when
      to_string($message.level) == "DEBUG"
    then
      drop_message();
    end
  EOT
}

# Note: Linking pipelines to streams in Graylog may require additional configuration
# (pipeline connections). This resource manages the pipeline definition itself.
