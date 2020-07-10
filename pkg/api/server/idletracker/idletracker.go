package idletracker

import (
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type IdleTracker struct {
	http     map[net.Conn]struct{}
	hijacked int
	total    int
	mux      sync.Mutex
	timer    *time.Timer
	Duration time.Duration
}

func NewIdleTracker(idle time.Duration) *IdleTracker {
	return &IdleTracker{
		http:     make(map[net.Conn]struct{}),
		Duration: idle,
		timer:    time.NewTimer(idle),
	}
}

func (t *IdleTracker) ConnState(conn net.Conn, state http.ConnState) {
	t.mux.Lock()
	defer t.mux.Unlock()

	oldActive := t.ActiveConnections()
	logrus.Debugf("IdleTracker %p:%v %d/%d connection(s)", conn, state, oldActive, t.TotalConnections())
	switch state {
	case http.StateNew, http.StateActive:
		t.http[conn] = struct{}{}
		// stop the timer if we transitioned from idle
		if oldActive == 0 {
			t.timer.Stop()
		}
		t.total++
	case http.StateHijacked:
		// hijacked connections are handled elsewhere
		delete(t.http, conn)
		t.hijacked++
	case http.StateIdle, http.StateClosed:
		delete(t.http, conn)
		// Restart the timer if we've become idle
		if oldActive > 0 && len(t.http) == 0 {
			t.timer.Stop()
			t.timer.Reset(t.Duration)
		}
	}
}

func (t *IdleTracker) TrackHijackedClosed() {
	t.mux.Lock()
	defer t.mux.Unlock()

	t.hijacked--
}

func (t *IdleTracker) ActiveConnections() int {
	return len(t.http) + t.hijacked
}

func (t *IdleTracker) TotalConnections() int {
	return t.total
}

func (t *IdleTracker) Done() <-chan time.Time {
	return t.timer.C
}
