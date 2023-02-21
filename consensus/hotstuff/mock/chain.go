package mock

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
)

func makeChain(db ethdb.Database, engine consensus.Engine, validators []common.Address) *core.BlockChain {
	genesis := makeGenesis(validators)
	block := genesis.MustCommit(db)
	log.Info("Make chain with genesis block", "hash", block.Hash())

	blockchain, _ := core.NewBlockChain(db, nil, genesis.Config, engine, vm.Config{}, nil, nil, nil)
	return blockchain
}
