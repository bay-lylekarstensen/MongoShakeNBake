#!/bin/sh
set -eu

CONF_INPUT="${MSB_CONF:-collector.conf}"

case "$CONF_INPUT" in
  */*)
    CONF_PATH="$CONF_INPUT"
    ;;
  *)
    CONF_PATH="/app/conf/$CONF_INPUT"
    ;;
esac

if [ "$#" -eq 0 ]; then
  exec /app/collector -conf="$CONF_PATH"
fi

case "$1" in
  -*)
    exec /app/collector -conf="$CONF_PATH" "$@"
    ;;
  collector|/app/collector)
    shift
    exec /app/collector -conf="$CONF_PATH" "$@"
    ;;
  receiver|/app/receiver)
    shift
    exec /app/receiver "$@"
    ;;
  *)
    exec "$@"
    ;;
esac
