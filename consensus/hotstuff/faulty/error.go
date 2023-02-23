package faulty

import "errors"

var (
	errUnauthorizedAddress = errors.New("unauthorized address")

	errInconsistentVote = errors.New("inconsistent vote")

	errInvalidDigest = errors.New("invalid digest")

	errNotFromProposer = errors.New("Message does not come from proposer")

	errNotToProposer = errors.New("Message does not send to proposer")

	errFutureMessage = errors.New("future Message")

	errFarAwayFutureMessage = errors.New("far away future Message")

	errOldMessage = errors.New("old Message")

	errFailedDecodeNewView = errors.New("failed to decode NEWVIEW")

	errFailedDecodePrepare = errors.New("failed to decode PREPARE")

	errFailedDecodePrepareVote = errors.New("failed to decode PREPARE_VOTE")

	errFailedDecodePreCommit = errors.New("failed to decode PRECOMMIT")

	errFailedDecodePreCommitVote = errors.New("faild to decode PRECOMMIT_VOTE")

	errFailedDecodeCommit = errors.New("failed to decode COMMIT")

	errFailedDecodeCommitVote = errors.New("failed to decode COMMIT_VOTE")

	errState = errors.New("error state")

	errNoRequest = errors.New("no valid request")

	errInvalidProposal = errors.New("invalid proposal")

	errVerifyUnsealedProposal = errors.New("verify unsealed proposal failed")

	errExtend = errors.New("proposal extend relationship error")

	errSafeNode = errors.New("safeNode checking failed")

	errAddNewViews = errors.New("add new view error")

	errAddPrepareVote = errors.New("add prepare vote error")

	errAddPreCommitVote = errors.New("add pre commit vote error")

	errInvalidSignature = errors.New("invalid signature")

	errUnauthorized = errors.New("unauthorized")

	errInvalidExtraDataFormat = errors.New("invalid extra data format")

	errInvalidCommittedSeals = errors.New("invalid committed seals")

	errEmptyCommittedSeals = errors.New("zero committed seals")

	errIncorrectAggInfo = errors.New("incorrect agg information")

	errInsufficientAggPub = errors.New("insufficient aggPub")

	errInvalidAggregatedSig = errors.New("invalid aggregated signature")

	errEmptyAggregatedSig = errors.New("zero aggregated signature")

	errInvalidProposalMyself = errors.New("invalid propsal, comes from myself")

	errTestIncorrectConversion = errors.New("incorrect conversion")

	errInvalidNode = errors.New("invalid node")

	errNilHighQC = errors.New("highQC is nil")

	errInvalidRawHash = errors.New("raw hash is invalid")

	errInvalidQC = errors.New("invalid qc")

	errFailedDecodeMessage = errors.New("message payload invalid")

	errInvalidCode = errors.New("message type invalid")

	errInvalidBlock = errors.New("invalid block")
)
