.PHONY: nodes-start nodes-stop clients-start clients-stop clean-all log-node-1 log-node-2 log-node3 log-client-1 log-client-2 gen-configs
export GOPRIVATE=github.com/anytypeio
# TODO: make different folders in etc for different programs

gen-configs:
	./init.sh config_gen

nodes-start:
	./init.sh nodes_start

nodes-stop:
	./init.sh nodes_stop

clean-all:
	rm -rf tmp

clients-start:
	./init.sh clients_start

clients-stop:
	./init.sh clients_stop

log-node-1:
	./init.sh node_log 1

log-node-2:
	./init.sh node_log 2

log-node-3:
	./init.sh node_log 3

log-client-1:
	./init.sh client_log 1

log-client-2:
	./init.sh client_log 2

debug:
	@(echo "To run debug api please run './init.sh debug [params]' with respective args to debug client")

run-all: nodes-start clients-start
	sleep 3000000

run-clean: clean-all run-all
