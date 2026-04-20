package server

import "time"

type secondTicker struct {
	t *time.Ticker
}

func newSecondTicker() *secondTicker { return &secondTicker{t: time.NewTicker(time.Second)} }
func (s *secondTicker) ch() <-chan time.Time { return s.t.C }
func (s *secondTicker) stop()                { s.t.Stop() }
