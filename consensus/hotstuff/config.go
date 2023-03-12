package hotstuff

type SelectProposerPolicy string

const (
	RoundRobin SelectProposerPolicy = "RoundRobin"
	Sticky     SelectProposerPolicy = "Sticky"
	VRF        SelectProposerPolicy = "VRF"
)

type FaultyMode string

const (
	Disabled             FaultyMode = "Disabled"             // Disabled disables the faulty mode
	TargetedBadPreCommit FaultyMode = "TargetedBadPreCommit" // Leader sends faulty PreCommit message to <=F replicas
	TargetedBadCommit    FaultyMode = "TargetedBadCommit"    // Leader sends faulty PreCommit message to <=F replicas
	BadDecide            FaultyMode = "BadDecide"            // Leader faulty-seals block but has a good decide
)

type Config struct {
	RequestTimeout uint64               `toml:",omitempty"` // The timeout for each HotStuff round in milliseconds.
	BlockPeriod    uint64               `toml:",omitempty"` // Default minimum difference between two consecutive block's timestamps in second for basic hotstuff and mill-seconds for event-driven
	LeaderPolicy   SelectProposerPolicy `toml:",omitempty"` // The policy for speaker selection
	FaultyMode     FaultyMode           `toml:",omitempty"` // The faulty node indicates the faulty node's behavior
}

var DefaultBasicConfig = &Config{
	RequestTimeout: 6000,
	BlockPeriod:    3,
	LeaderPolicy:   RoundRobin,
	FaultyMode:     Disabled,
}
