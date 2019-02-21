// +build windows

package mpb

import (
	"time"
)

func (p *Progress) serve(s *pState) {

	var ticker *time.Ticker
	var refreshCh <-chan time.Time

	if s.manualRefreshCh == nil {
		ticker = time.NewTicker(s.rr)
		refreshCh = ticker.C
	} else {
		refreshCh = s.manualRefreshCh
	}

	for {
		select {
		case op := <-p.operateState:
			op(s)
		case <-refreshCh:
			if s.zeroWait {
				if s.manualRefreshCh == nil {
					ticker.Stop()
				}
				if s.shutdownNotifier != nil {
					close(s.shutdownNotifier)
				}
				close(p.done)
				return
			}
			tw, err := s.cw.GetWidth()
			if err != nil {
				tw = s.width
			}
			s.render(tw)
		}
	}
}
