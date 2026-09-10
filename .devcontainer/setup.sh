#!/bin/bash
set -e

if [ -d .git ]; then
    git pull || true
fi

go mod download

# GPG signing requires necessary setting enabled in GitHub.
# See https://docs.github.com/en/codespaces/managing-your-codespaces/managing-gpg-verification-for-github-codespaces
git config --global commit.gpgsign false

go build ./...