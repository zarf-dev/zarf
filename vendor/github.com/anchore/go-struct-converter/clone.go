package converter

func Clone(from, to any) error {
	return NewFuncChain().AllowImplicit().Convert(from, to)
}
