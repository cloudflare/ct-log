package ct

import (
	"fmt"
)

type fsmState int

const (
	sBegin fsmState = iota
	sQueueLeaves
	sDequeueLeaves
	sUpdateSequencedLeaves
	sSetMerkleNodes
	sStoreSignedLogRoot
	sCommit
	sRollback
	sClose
)

func (s fsmState) String() string {
	switch s {
	case sBegin:
		return "BeginForTree"
	case sQueueLeaves:
		return "QueueLeaves"
	case sDequeueLeaves:
		return "DequeueLeaves"
	case sUpdateSequencedLeaves:
		return "UpdateSequencedLeaves"
	case sSetMerkleNodes:
		return "SetMerkleNodes"
	case sStoreSignedLogRoot:
		return "StoreSignedLogRoot"
	case sCommit:
		return "Commit"
	case sRollback:
		return "Rollback"
	case sClose:
		return "Close"
	default:
		return fmt.Sprintf("<unknown state: %v>", int(s))
	}
}

var fsmTransitions = map[fsmState][]fsmState{
	sBegin:                 {sQueueLeaves, sDequeueLeaves, sStoreSignedLogRoot},
	sQueueLeaves:           {sCommit},
	sDequeueLeaves:         {sUpdateSequencedLeaves, sCommit},
	sUpdateSequencedLeaves: {sSetMerkleNodes},
	sSetMerkleNodes:        {sStoreSignedLogRoot},
	sStoreSignedLogRoot:    {sCommit},
	sCommit:                {sClose},
	sRollback:              {sClose},
}

// fsm is used by a logTreeTX to prevent Trillian from modifying the databases
// in an unexpected ways, by modeling valid transactions as finite-state
// machines.
type fsm struct {
	state fsmState
}

func (m *fsm) emit(state fsmState) error {
	if state == sRollback || state == sClose {
		m.state = state
		return nil
	}

	for _, cand := range fsmTransitions[m.state] {
		if state == cand {
			m.state = state
			return nil
		}
	}
	return fmt.Errorf("illegal transition blocked: %v -> %v", m.state, state)
}
