package hotstuff

import "errors"

var (
	ErrInvalidMessage = errors.New("invalid Message")

	ErrInvalidSigner = errors.New("Message not signed by the sender")

	ErrUnauthorizedAddress = errors.New("unauthorized address")

	ErrInconsistentVote = errors.New("inconsistent vote")

	ErrInvalidDigest = errors.New("invalid digest")

	ErrNotFromProposer = errors.New("Message does not come from proposer")

	ErrNotToProposer = errors.New("Message does not send to proposer")

	ErrFutureMessage = errors.New("future Message")

	ErrFarAwayFutureMessage = errors.New("far away future Message")

	ErrOldMessage = errors.New("old Message")

	ErrFailedDecodeNewView = errors.New("failed to decode NEWVIEW")

	ErrFailedDecodePrepare = errors.New("failed to decode PREPARE")

	ErrFailedDecodePrepareVote = errors.New("failed to decode PREPARE_VOTE")

	ErrFailedDecodePreCommit = errors.New("failed to decode PRECOMMIT")

	ErrFailedDecodePreCommitVote = errors.New("faild to decode PRECOMMIT_VOTE")

	ErrFailedDecodeCommit = errors.New("failed to decode COMMIT")

	ErrFailedDecodeCommitVote = errors.New("failed to decode COMMIT_VOTE")

	ErrState = errors.New("error state")

	ErrNoRequest = errors.New("no valid request")

	ErrVerifyUnsealedProposal = errors.New("verify unsealed proposal failed")

	ErrExtend = errors.New("proposal extend relationship error")

	ErrSafeNode = errors.New("safeNode checking failed")

	ErrAddNewViews = errors.New("add new view error")

	ErrAddPrepareVote = errors.New("add prepare vote error")

	ErrAddPreCommitVote = errors.New("add pre commit vote error")

	ErrInvalidSignature = errors.New("invalid signature")

	ErrIncorrectAggInfo = errors.New("incorrect agg information")

	ErrInsufficientAggPub = errors.New("insufficient aggPub")

	ErrInvalidAggregatedSig = errors.New("invalid aggregated signature")

	ErrEmptyAggregatedSig = errors.New("zero aggregated signature")

	ErrInvalidProposalMyself = errors.New("invalid propsal, comes from myself")

	ErrTestIncorrectConversion = errors.New("incorrect conversion")

	ErrInvalidNode = errors.New("invalid node")

	ErrNilHighQC = errors.New("highQC is nil")

	ErrInvalidRawHash = errors.New("raw hash is invalid")

	ErrInvalidQC = errors.New("invalid qc")

	ErrFailedDecodeMessage = errors.New("message payload invalid")

	ErrInvalidCode = errors.New("message type invalid")

	ErrInvalidBlock = errors.New("invalid block")

	// ErrInvalidProposal is returned when a prposal is malformed.
	ErrInvalidProposal = errors.New("invalid proposal")
	// ErrUnknownBlock is returned when the list of validators is requested for a block that is not part of the local blockchain.
	ErrUnknownBlock = errors.New("unknown block")
	// ErrUnauthorized is returned if a header is signed by a non authorized entity.
	ErrUnauthorized = errors.New("unauthorized")
	// ErrInvalidDifficulty is returned if the difficulty of a block is not 1
	ErrInvalidDifficulty = errors.New("invalid difficulty")
	// ErrInvalidExtraDataFormat is returned when the extra data format is incorrect
	ErrInvalidExtraDataFormat = errors.New("invalid extra data format")
	// ErrInvalidMixDigest is returned if a block's mix digest is not Istanbul digest.
	ErrInvalidMixDigest = errors.New("invalid Istanbul mix digest")
	// ErrInvalidUncleHash is returned if a block contains an non-empty uncle list.
	ErrInvalidUncleHash = errors.New("non empty uncle hash")
	// ErrInvalidTimestamp is returned if the timestamp of a block is lower than the previous block's timestamp + the minimum block period.
	ErrInvalidTimestamp = errors.New("invalid timestamp")
	// ErrInvalidCommittedSeals is returned if the committed seal is not signed by any of parent validators.
	ErrInvalidCommittedSeals = errors.New("invalid committed seals")
	// ErrEmptyCommittedSeals is returned if the field of committed seals is zero.
	ErrEmptyCommittedSeals = errors.New("zero committed seals")
	// ErrMismatchTxhashes is returned if the TxHash in header is mismatch.
	ErrMismatchTxhashes = errors.New("mismatch transactions hashes")
	// ErrDecodeFailed is returned if the message can't be decode
	ErrDecodeFailed = errors.New("decode p2p message failed")
)
