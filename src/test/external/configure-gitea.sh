#!/usr/bin/env bash

set -euo pipefail

# Retry gitea migrate until the db is ready
timeout 30 bash -c 'until gitea migrate; do sleep 2; done'

echo '==== BEGIN GITEA CONFIGURATION ===='

function configure_admin_user() {
    local ACCOUNT_ID=$(gitea admin user list --admin | grep -e "\s\+${GITEA_ADMIN_USERNAME}\s\+" | awk -F " " "{printf \$1}")
    if [[ -z "${ACCOUNT_ID}" ]]; then
    echo "No admin user '${GITEA_ADMIN_USERNAME}' found. Creating now..."
    gitea admin user create --admin --username "${GITEA_ADMIN_USERNAME}" --password "${GITEA_ADMIN_PASSWORD}" --email "${GITEA_ADMIN_EMAIL}" --must-change-password=false
    echo '...created.'
    else
    echo "Admin account '${GITEA_ADMIN_USERNAME}' already exist. Running update to sync password..."
    gitea admin user change-password --username "${GITEA_ADMIN_USERNAME}" --password "${GITEA_ADMIN_PASSWORD}"
    echo '...password sync done.'
    fi
}

configure_admin_user

echo '==== END GITEA CONFIGURATION ===='
