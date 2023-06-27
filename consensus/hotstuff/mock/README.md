# BHS Mock Network

`hotstuff/mock` was created for functional testing our BHS implementation. It works by imitating a GoQuorum/geth `backend` while retaining the same `core` logic. This module ensures that BHS can at least detect simple Byzantine behaviors w/ tampered fields.

Tests and the mock system were largely adopted from [`polynetwork/Zion`](https://github.com/polynetwork/Zion).

## Functional Testing

Functional tests are located in `mock_*_test.go` files. Faulty behavior is defined by a `hook`, which tampers with validator data right before sending (see `MockPeer.Send()`). Round changes are indicative of whether a faulty behavior was detected or not. Thus, tests pass or fail whether the faulty behavior causes a round change or not.

All tests follow the naming convention `Test<MsgType><TestType>`.

## Leader-to-Replica Phase Tests

Leader-to-replica (L-R) tests run in a 4-validator network that tolerate at least 1 bad node (`N=4, F=1`).   

### `Prepare`, `PreCommit`, `Commit` Messages

For these messages, we inject faults into the ff fields:

- `Message.View.Height`
- `Message.View.Round`
- `QuorumCert.View.Height`
- `QuorumCert.View.Round`
- `QuorumCert.ProposedBlock`
- `QuorumCert.BLSSignature`

A test passes if, after injecting the fault, a round change occurs.

### `Decide` Messages

`Decide` messages are special in that they send both the commitQC and proposed block hash (as the `Diploma` data structure). We lightly tests both of these fields, adding more tests in our `hotstuff/faulty` module. We test the ff fields:

- `Message.View.Height`
- `Message.View.Round`
- `Diploma.BlockHash`
- `Diploma.CommitQC.ProposedBlock`

A test passes if, after injecting the fault, a round change occurs.


## Replica-to-Leader Phase Tests

Most replica-to-leader tests run in a 4-validator network that tolerate at least 1 bad node (`N=4, F=1`).

### `PrepareVote`, `PreCommitVote`, `CommitVote` Messages

For these messages, we test both (i) fault detection and (ii) fault-tolerance below threshold `F`. Fault-detection is checked by sending `> F` faulty messages. Fault-tolerance is checked by sending `< F` faulty messages. We test the ff fields:

- `Vote.View.Height`
- `Vote.View.Round`
- `Message.Msg`

Tests for (i) pass if a round change occurs. Tests for (ii) pass if a round change does not occur.

### `NewView` Messages

NewView message send both a QC and signal the start of a new round. As such, they require more testing. We test the ff fields for fault-detection:

- `Message.View.Round` (`Height` testing not included)
- `QuorumCert.ProposedBlock`
- `QuorumCert.View.Height`
- `QuorumCert.View.Round`
- `QuorumCert.BLSSignature`

Additionally, we test when a `NewView` message is sent to the wrong leader.

Tests pass if a round change does not occur.

## Limitations

To best of our knowledge, this merely checks for simple Byzantine faults. We do not check for complex collusion strategies. Additionally, `Vote` fields are not checked.
