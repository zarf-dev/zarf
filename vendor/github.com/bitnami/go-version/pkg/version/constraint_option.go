package version

type conf struct {
	zeroPadding bool
}

type ConstraintOption interface {
	apply(*conf)
}

type WithZeroPadding bool

func (o WithZeroPadding) apply(c *conf) {
	c.zeroPadding = bool(o)
}
