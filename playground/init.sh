#!/bin/bash
PATH=/bin:/sbin:/usr/bin:/usr/sbin:/Users/mikhailrakhmanov/go/go1.19.2/bin

export GOPRIVATE=github.com/anytypeio

NODE_GO="../../../node/cmd/node.go"
CLIENT_GO="../../../client/cmd/client.go"
CONFIGS_DIR="../../../etc/configs"

do_usage() {
  echo "usage: $0 {nodes_start|nodes_stop|clients_start|clients_stop}"
  echo "usage: $0 node_log <N>"
  echo "usage: $0 client_log <N>"
  exit 1
}

do_nodes_start() {
  for NUMBER in {1..3}; do
    install -d tmp/node$NUMBER/ tmp/log/
    (cd tmp/node$NUMBER && go run $NODE_GO -c $CONFIGS_DIR/node$NUMBER.yml &>../log/node$NUMBER.log) &
    NODE_PID=$!
    echo $NODE_PID >tmp/node$NUMBER.pid
    echo NODE_PID=$NODE_PID
  done
}

do_nodes_stop() {
  for NUMBER in {1..3}; do
    NODE_PID=$(cat tmp/node$NUMBER.pid)
    pkill -P $NODE_PID
    pkill -f $CONFIGS_DIR/node$NUMBER.yml
  done
}

do_node_log() {
  local NODE_NUMBER=$1
  tail -f tmp/log/node$NODE_NUMBER.log
}

do_client_log() {
  local CLIENT_NUMBER=$1
  tail -f tmp/log/client$CLIENT_NUMBER.log
}

do_clients_start() {
  for NUMBER in {1..2}; do
    install -d tmp/client$NUMBER/ tmp/log/
    (cd tmp/client$NUMBER && go run $CLIENT_GO -c $CONFIGS_DIR/client$NUMBER.yml &>../log/client$NUMBER.log) &
    CLIENT_PID=$!
    echo $CLIENT_PID >tmp/client$NUMBER.pid
    echo CLIENT_PID=$CLIENT_PID
  done
}

do_clients_stop() {
  for NUMBER in {1..2}; do
    CLIENT_PID=$(cat tmp/client$NUMBER.pid)
    pkill -P $CLIENT_PID
    pkill -f $CONFIGS_DIR/client$NUMBER.yml
  done
}

case $1 in
nodes_start | nodes_stop | clients_start | clients_stop)
  do_$1
  ;;
node_log)
  if [[ -z $2 ]]; then
    do_usage
  else
    do_$1 $2
  fi
  ;;
client_log)
  if [[ -z $2 ]]; then
    do_usage
  else
    do_$1 $2
  fi
  ;;
*)
  do_usage
  ;;
esac
