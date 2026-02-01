---
page_title: "graylog_ldap_setting Resource - Graylog Terraform Provider"
subcategory: "Security"
description: |-
  Terraform Graylog provider: manage global LDAP settings in Graylog (singleton). Keywords: terraform graylog provider, graylog terraform, terraform graylog, graylog automation, Graylog operation automation. Управляет глобальными настройками LDAP в Graylog (singleton ресурс).
---

# graylog_ldap_setting

Ресурс управляет глобальными настройками LDAP в Graylog. Это одиночный ресурс (singleton).

## Example Usage

```
resource "graylog_ldap_setting" "this" {
  enabled       = false
  ldap_uri      = "ldap://ldap.example.org:389"
  search_base   = "dc=example,dc=org"
  search_pattern = "(uid={0})"
}
```

## Argument Reference

- `enabled` — (Optional) Включить LDAP-аутентификацию.
- `system_username` — (Optional) Bind DN / системное имя пользователя.
- `system_password` — (Optional, Sensitive) Пароль для bind.
- `ldap_uri` — (Optional) URI LDAP.
- `search_base` — (Optional) Базовый DN для поиска пользователей.
- `search_pattern` — (Optional) Паттерн поиска пользователей.
- `user_unique_id_attribute` — (Optional) Атрибут уникального идентификатора.
- `group_search_base` — (Optional) Базовый DN для поиска групп.
- `group_search_pattern` — (Optional) Паттерн поиска групп.
- `default_group` — (Optional) Группа по умолчанию в Graylog.
- `use_start_tls` — (Optional) Использовать STARTTLS.
- `trust_all_certificates` — (Optional) Доверять всем сертификатам.
- `active_directory` — (Optional) Режим Active Directory.
- `display_name_attribute` — (Optional) Атрибут отображаемого имени.
- `email_attribute` — (Optional) Атрибут email.

## Attributes Reference

- `id` — фиксированное значение `ldap`.

## Import

```
terraform import graylog_ldap_setting.this ldap
```
