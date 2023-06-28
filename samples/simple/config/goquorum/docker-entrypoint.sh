#!/bin/sh

set -o errexit
set -o nounset
set -o pipefail

GOQUORUM_CONS_ALGO=`echo "${GOQUORUM_CONS_ALGO:-qbft}" | tr '[:lower:]'`
BLOCK_PERIOD=`echo "${BLOCK_PERIOD}"`
GENESIS_FILE=${GENESIS_FILE:-"/data/genesis.json"}

cp -R /config/* /data
mkdir -p /data/keystore/

echo "Applying ${GENESIS_FILE} ..."
geth --nousb --verbosity 1 --datadir=/data init ${GENESIS_FILE}; 

cp /config/keys/accountKeystore /data/keystore/key;
cp /config/keys/nodekey /data/geth/nodekey;

if [ "ibft" == "$GOQUORUM_CONS_ALGO" ];
then
    echo "Using istanbul for consensus algorithm..."
    export CONSENSUS_ARGS="--istanbul.blockperiod $BLOCK_PERIOD --mine --miner.threads 1 --miner.gasprice 0 --emitcheckpoints"
    export QUORUM_API="istanbul"
elif [ "qbft" == "$GOQUORUM_CONS_ALGO" ];
then
    echo "Using qbft for consensus algorithm..."
    export CONSENSUS_ARGS="--mine --miner.threads 1 --miner.gasprice 0 --emitcheckpoints"
    export QUORUM_API="istanbul"
elif [ "raft" == "$GOQUORUM_CONS_ALGO" ];
then
    echo "Using raft for consensus algorithm..."
    export CONSENSUS_ARGS="--raft --raftblocktime 300 --raftport 53000"
    export QUORUM_API="raft"
elif [ "hotstuff" == "$GOQUORUM_CONS_ALGO" ];
then
    echo "Using hotstuff for consensus algorithm..."
    cp /config/keys/bls-private-key.json /data;
    cp /config/keys/bls-public-key.json /data;
    export CONSENSUS_ARGS="--mine --miner.threads 1 --miner.gasprice 0"
    export QUORUM_API="hotstuff"
fi

export ADDRESS=$(grep -o '"address": *"[^"]*"' /config/keys/accountKeystore | grep -o '"[^"]*"$' | sed 's/"//g')

touch /var/log/quorum/geth-$(hostname -i).log
cat /proc/1/fd/2 /proc/1/fd/1 > /var/log/quorum/geth-$(hostname -i).log &

if [ -f /permissions/permission-config.json ]; then
  echo "Using Enhanced Permissions: Copying permission-config.json exists to /data ...";
  cp /permissions/permission-config.json /data/permission-config.json
fi

exec geth \
--datadir /data \
--nodiscover \
--verbosity 5 --log.json \
$CONSENSUS_ARGS \
--syncmode full --revertreason \
--metrics --pprof --pprof.addr 0.0.0.0 --pprof.port 9545 \
--networkid ${QUORUM_NETWORK_ID:-1337} \
--http --http.addr 0.0.0.0 --http.port 8545 --http.corsdomain "*" --http.vhosts "*" --http.api admin,eth,debug,miner,net,txpool,personal,web3,$QUORUM_API \
--ws --ws.addr 0.0.0.0 --ws.port 8546 --ws.origins "*" --ws.api admin,eth,debug,miner,net,txpool,personal,web3,$QUORUM_API \
--port 30303 \
--identity ${HOSTNAME}-${GOQUORUM_CONS_ALGO} \
--unlock ${ADDRESS} \
--allow-insecure-unlock \
--password /data/passwords.txt \
${ADDITIONAL_ARGS:-} \
2>&1