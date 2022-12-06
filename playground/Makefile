.PHONY: client1 client2 node nodes-start nodes-stop clean-db log-node-1 log-node-2 log-node3 log-client-1 log-client-2
export GOPRIVATE=github.com/anytypeio
# TODO: make different folders in etc for different programs

nodes-start:
	./init.sh nodes_start

nodes-stop:
	./init.sh nodes_stop

clean-db:
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