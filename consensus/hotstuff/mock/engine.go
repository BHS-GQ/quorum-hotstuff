package mock

import (
	"crypto/ecdsa"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/hotstuff"
	"github.com/ethereum/go-ethereum/consensus/hotstuff/backend"
	"github.com/ethereum/go-ethereum/consensus/hotstuff/validator"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
)

type Engine consensus.Engine

// backend is engine but also hotstuff engine and consensus handler.
func makeEngine(
	privateKey *ecdsa.PrivateKey,
	db ethdb.Database,
	validators []common.Address,
	blsInfo *types.BLSInfo,
) Engine {
	config := hotstuff.DefaultBasicConfig
	valset := validator.NewSet(validators, hotstuff.RoundRobin)
	engine := backend.New(config, privateKey, db, valset, blsInfo)
	broadcaster := makeBroadcaster(engine.Address(), engine)
	engine.SetBroadcaster(broadcaster)
	return engine
}
