package progress

import "time"

type Generator struct {
	monitor Monitorable
	sizer   Sizable
	last    int64
	timer   TimeEstimator
}

func NewGenerator(monitor Monitorable, sizer Sizable) *Generator {
	return &Generator{
		monitor: monitor,
		sizer:   sizer,
		timer:   NewTimeEstimator(),
	}
}

func (g *Generator) Progress() Progress {
	result := Progress{
		current: g.monitor.Current(),
		size:    g.sizer.Size(),
		err:     g.monitor.Error(),
	}

	if g.last == 0 && result.current > 0 {
		g.timer.Start()
	} else {
		g.last = result.current
	}

	g.timer.Update(result)
	return result
}

func (g *Generator) Remaining() time.Duration {
	return g.timer.Remaining()
}

func (g *Generator) Estimated() time.Time {
	return g.timer.Estimated()
}
