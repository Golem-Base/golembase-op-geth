op-geth: bash -c "rm -rf /tmp/golembase.wal/*; exec go run ./cmd/geth --dev --http --http.api 'eth,web3,net,debug,golembase' --verbosity 3 --http.addr '0.0.0.0' --http.port 8545 --http.corsdomain '*' --http.vhosts '*' --golembase.writeaheadlog '/tmp/golembase.wal'"
sqlite-etl: bash -c "rm /tmp/golembase-sqlite*; sleep 10s; exec go run ./golem-base/etl/sqlite/ --wal '/tmp/golembase.wal' --db /tmp/golembase-sqlite --rpc-endpoint http://localhost:8545"
mongodb: ./golem-base/script/run-mongo-in-docker.sh
mongodb-etl: ./golem-base/script/start-mongodb-etl.sh
