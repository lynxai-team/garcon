// Copyright 2021 The contributors of Garcon.
// This file is part of Garcon, a static web builder, API server and middleware using Git, docker and podman.
// SPDX-License-Identifier: MIT

package gc

import (
	"time"
)

// Muter can be used to limit the logger/alerting verbosity.
// Muter detects when its internal counter is over the Threshold
// and waits the counter goes back to zero or after NoAlertDuration
// to return to normal situation.
// Muter uses the Hysteresis principle: https://wikiless.org/wiki/Hysteresis
// Similar wording: quieter, stopper, limiter, reducer, inhibitor, mouth-closer.
type Muter struct {
	// quietTime is the first call of successive Decrement()
	// without any Increment(). quietTime is used to
	// inform the time since no Increment() has been called.
	quietTime time.Time

	// NoAlertDuration allows to consider the verbosity is back to normal.
	NoAlertDuration time.Duration

	// Threshold is the level enabling the muted state.
	Threshold int

	// RemindMuteState allows to remind the state is still muted.
	// Set value 100 to send this reminder every 100 increments.
	// Set to zero to disable this feature,
	RemindMuteState int

	// counter is incremented/decremented, but is never negative.
	counter int

	// dropped is the number of Increment() calls after state became muted.
	dropped int

	// muted represent the Muter state.
	muted bool
}

// Increment increments the internal counter and returns false when in muted state.
// Every RemindMuteState calls, Increment also returns the number of times Increment has been called.
func (m *Muter) Increment() (ok bool, dropped int) {
	m.counter++

	if m.muted {
		m.dropped++
		if (m.RemindMuteState == 0) || (m.dropped%m.RemindMuteState) > 0 {
			return false, -1
		}
	}

	if m.muted {
		return true, m.dropped
	}

	if m.counter > m.Threshold {
		m.muted = true
		m.dropped = 0
		m.quietTime = time.Time{}
		return true, 1
	}

	return true, 0
}

// Decrement decrements the internal counter and switches to un-muted state
// when counter reaches zero or after NoAlertDuration.
func (m *Muter) Decrement() (ok bool, _ time.Time, dropped int) {
	if !m.muted {
		return false, time.Time{}, 0 // already un-muted, do nothing
	}

	m.counter--
	if m.counter > 0 {
		if m.quietTime.IsZero() {
			// first call to Decrement() since last Increment() call
			m.quietTime = time.Now()
			return false, time.Time{}, 0
		}

		if time.Since(m.quietTime) < m.NoAlertDuration {
			return false, time.Time{}, 0
		}

		m.counter = 0
	}

	m.muted = false

	return true, m.quietTime, m.dropped
}
