#!/usr/bin/env bash

set -euo pipefail

if [ "$#" -ne 2 ]; then
  echo "usage: $0 <terraform-directory> <step-label>" >&2
  exit 2
fi

terraform_dir=$1
step_label=$2
plan_file="migration-${step_label}.tfplan"
apply_log="migration-${step_label}.apply.log"

rm -f "${terraform_dir}/${plan_file}" "${terraform_dir}/${apply_log}"

set +e
terraform -chdir="${terraform_dir}" plan -detailed-exitcode -out="${plan_file}"
plan_exit=$?
set -e

if [ "${plan_exit}" -ne 0 ] && [ "${plan_exit}" -ne 2 ]; then
  echo "${step_label}: terraform plan failed with exit code ${plan_exit}" >&2
  exit "${plan_exit}"
fi

destructive_changes=$(
  terraform -chdir="${terraform_dir}" show -json "${plan_file}" |
    jq -r '
      .resource_changes[]?
      | select(any(.change.actions[]?; . == "delete"))
      | "\(.address): \(.change.actions | join(" -> "))"
    '
)

if [ -n "${destructive_changes}" ]; then
  echo "${step_label}: migration plan contains destructive resource actions:" >&2
  printf '%s\n' "${destructive_changes}" >&2
  echo "Refusing to apply: migration must preserve every resource already in state." >&2
  exit 1
fi

terraform -chdir="${terraform_dir}" apply -auto-approve "${plan_file}" |
  tee "${terraform_dir}/${apply_log}"

if grep -Eq '(^|[[:space:]])Destroying\.\.\.|Destruction complete after|Destroy complete!' "${terraform_dir}/${apply_log}"; then
  echo "${step_label}: terraform apply performed a destroy despite the plan guard" >&2
  exit 1
fi

rm -f "${terraform_dir}/${plan_file}" "${terraform_dir}/${apply_log}"
