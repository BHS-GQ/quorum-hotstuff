package mock

import (
	"math/big"
	"os"

	"github.com/ethereum/go-ethereum/common"
	hs "github.com/ethereum/go-ethereum/consensus/hotstuff"
	"github.com/ethereum/go-ethereum/consensus/hotstuff/validator"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
)

const (
	EpochStart = uint64(0)
	EpochEnd   = uint64(10000000000)

	hotstuffMsg = 0x11
)

func init() {
	glogger := log.NewGlogHandler(log.StreamHandler(os.Stderr, log.TerminalFormat(false)))
	glogger.Verbosity(log.LvlTrace)
	log.Root().SetHandler(glogger)
}

func makeGenesis(vals []common.Address) *core.Genesis {
	genesis := &core.Genesis{
		Config: &params.ChainConfig{
			ChainID:             big.NewInt(60801),
			HomesteadBlock:      big.NewInt(0),
			EIP150Block:         big.NewInt(0),
			EIP155Block:         big.NewInt(0),
			EIP158Block:         big.NewInt(0),
			ByzantiumBlock:      big.NewInt(0),
			ConstantinopleBlock: big.NewInt(0),
			PetersburgBlock:     big.NewInt(0),
			IstanbulBlock:       big.NewInt(0),
			BerlinBlock:         big.NewInt(0),
			HotStuff:            &params.HotStuffConfig{},
		},
		Coinbase:   common.HexToAddress("0x0000000000000000000000000000000000000000"),
		Difficulty: big.NewInt(1),
		GasLimit:   2097151,
		Nonce:      4976618949627435365,
		Mixhash:    common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000"),
		ParentHash: common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000"),
		Timestamp:  0,
	}

	valset := validator.NewSet(vals, hs.RoundRobin)
	genesis.ExtraData, _ = types.GenerateExtraWithSignature(EpochStart, EpochEnd, valset.AddressList(), []byte{}, []byte{})
	return genesis
}
