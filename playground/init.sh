#!/bin/bash

export GOPRIVATE=github.com/anytypeio

NODE_GO="../../../node/cmd/node.go"
CLIENT_GO="../../../client/cmd/client.go"
DEBUG_GO="../util/cmd/debug/debug.go"
NODEMAP_YML="../util/cmd/nodesgen/nodemap.yml"
CONFIGS_DIR="../../../etc/configs"
NODESGEN_GO="../util/cmd/nodesgen/gen.go"
ETC_DIR="../etc"

do_usage() {
  echo "usage: $0 {nodes_start|nodes_stop|clients_start|clients_stop|debug|config_gen}"
  echo "usage: $0 node_log <N>"
  echo "usage: $0 client_log <N>"
  exit 1
}

do_nodes_start() {
  for NUMBER in {1..3}; do
    install -d tmp/node$NUMBER/ tmp/log/
    export ANYPROF="127.0.0.1:607$NUMBER"
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
  tail -f -n 500 tmp/log/node$NODE_NUMBER.log
}

do_client_log() {
  local CLIENT_NUMBER=$1
  tail -f -n 500 tmp/log/client$CLIENT_NUMBER.log
}

do_config_gen() {
  go run $NODESGEN_GO -n $NODEMAP_YML -e $ETC_DIR
  cp -rf $ETC_DIR/configs/node1.yml $ETC_DIR/config.yml
  cp -rf $ETC_DIR/configs/client1.yml $ETC_DIR/client.yml
}

do_clients_start() {
  for NUMBER in {1..2}; do
    export ANYPROF="127.0.0.1:606$NUMBER"
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

do_debug() {
  go run $DEBUG_GO --config $NODEMAP_YML $*
}

case $1 in
nodes_start | nodes_stop | clients_start | clients_stop | config_gen)
  do_$1
  ;;
debug)
  first_arg=$1
  shift
  do_$first_arg $*
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
