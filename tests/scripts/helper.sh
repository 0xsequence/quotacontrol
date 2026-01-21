#!/bin/bash

CONFIG_FILE=${CONFIG_FILE:-tests/server/config.json}
PID_FILE="server.pid"
LOGS="logs/server.log"
PORT=$(jq -r '.URL | split(":") | .[-1]' "${CONFIG_FILE}")

# ANSI color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

main() {
  case "${1}" in
    "server")
      case "${2}" in
        start)
          server-start
          ;;
        status)
          server-status
          ;;
        stop)
          server-stop
          ;;
        kill)
          server-kill
          ;;
        restart)
          server-restart
          ;;
        *)
          server-usage
          ;;
      esac
      ;;
    "client")
      echo -e "${RED}QuotaControl:${NC} client commands are not implemented in this script."
      exit 1
      ;;
  esac
}

server-usage() {
  echo -e "${RED}QuotaControl:${NC} usage: $0 {start|status|stop|kill|restart}"
  exit 1
}

server-start() {
  server-status "" quiet && { echo -e "${RED}QuotaControl:${NC} server is already running."; exit 1; }
  echo -e "${BLUE}QuotaControl:${NC} starting server on port ${PORT}..."
  mkdir -p "$(dirname "${LOGS}")"
  PID=$(bin/server > $LOGS 2>&1 & echo $!)
  sleep 0.5
  server-status $PID || { echo -e "${RED}QuotaControl:${NC} failed to start server.\n\n$LOGS\n---"; cat $LOGS; exit 1; }
  echo "${PID}" > "${PID_FILE}"
}

server-status() {
  local pid="${1}"
  local quiet="${2}"
  if [ -z "${pid}" ]; then
  [ -f "${PID_FILE}" ] && pid=$(cat "${PID_FILE}") || { [ -z "${quiet}" ] && echo -e "${YELLOW}QuotaControl:${NC} server is not running."; return 1; }
  fi
  if kill -0 "${pid}" 2>/dev/null; then
  [ -z "${quiet}" ] && echo -e "${GREEN}QuotaControl:${NC} server is running with PID ${pid}."
  return 0
  else
  [ -z "${quiet}" ] && echo -e "${YELLOW}QuotaControl:${NC} server is not running."
  return 1
  fi
}

server-stop() {
  PID=$(cat "${PID_FILE}" 2>/dev/null)
  if [ -z "${PID}" ]; then
    echo -e "${YELLOW}QuotaControl:${NC} server is not running."
    return 1
  fi
  echo -e "${BLUE}QuotaControl:${NC} stopping server with PID ${PID}..."
  kill $(cat "${PID_FILE}") 2>/dev/null || { echo -e "${RED}QuotaControl:${NC} failed to stop server."; return 1; }
  rm server.pid
  echo -e "${GREEN}QuotaControl:${NC} server stopped."
}

server-kill() {
  PID=$(lsof -ti:${PORT})
  if [ -n "${PID}" ]; then
    echo -e "${BLUE}QuotaControl:${NC} killing server process ${PID}..."
    kill -9 ${PID}
    echo -e "${GREEN}QuotaControl:${NC} Process ${PID} killed."
  else
    echo -e "${YELLOW}QuotaControl:${NC} no process found on port ${PORT}."
  fi
}

server-restart() {
  server-stop
  sleep 0.5
  server-start
}

main "$@"
