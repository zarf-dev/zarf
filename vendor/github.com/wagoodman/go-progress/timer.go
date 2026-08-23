package progress

import "time"

type TimeEstimator struct {
	start     time.Time
	estimated time.Time
}

func NewTimeEstimator() TimeEstimator {
	return TimeEstimator{}
}

func (t *TimeEstimator) Start() {
	t.start = time.Now()
}

func (t *TimeEstimator) Update(p Progress) {
	ratio := float64(p.current) / float64(p.size)
	elapsed := float64(time.Since(t.start))
	if p.current > 0 {
		t.estimated = t.start.Add(time.Duration(elapsed / ratio))
	}
}

func (t *TimeEstimator) Remaining() time.Duration {
	if t.estimated.IsZero() {
		return -1
	}
	return time.Until(t.estimated)
}

func (t *TimeEstimator) Estimated() time.Time {
	return t.estimated
}
