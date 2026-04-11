#!/bin/sh

set -eu

config_path=""
prev=""

for arg in "$@"; do
	if [ "$prev" = "--config" ]; then
		config_path="$arg"
		break
	fi
	case "$arg" in
		--config=*)
			config_path="${arg#--config=}"
			break
			;;
	esac
	prev="$arg"
done

if [ -z "$config_path" ]; then
	config_path="/etc/gonetsim/gonetsim.toml"
fi

if [ "${1:-}" != "tls" ]; then
	su-exec gonetsim /gonetsim tls --config "$config_path" >/dev/null

	ca_pem_dir="$(dirname "$config_path")"
	if [ -f "$ca_pem_dir/gonetsim-ca.pem" ]; then
		cp "$ca_pem_dir/gonetsim-ca.pem" /usr/local/share/ca-certificates/gonetsim-ca.crt
		update-ca-certificates >/dev/null
	fi
fi

exec su-exec gonetsim /gonetsim "$@"