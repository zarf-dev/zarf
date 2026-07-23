package gocvss20

// The following values are the enumerations of each metric possible values.
// Do not change their order as it is vital to the implementation.
//
// Those are used to get a highly memory-performant implementation.
// The 4 bytes are used as follows, using the uint8 type.
//
//    u0       u1       u2       u3
// /------\ /------\ /------\ /------\
// ........ ........ ........ ........
// \/\/\/\/ \/\/\_/\__/\/\_/\__/\/\/\/
// AV |Au C  I A E  RL RC |  TD CR |AR
//   AC                  CDP      IR

// Base

const (
	av_l uint8 = iota
	av_a
	av_n
)

const (
	ac_l uint8 = iota
	ac_m
	ac_h
)

const (
	au_m uint8 = iota
	au_s
	au_n
)

const (
	cia_n uint8 = iota
	cia_p
	cia_c
)

// Temporal

const (
	e_nd uint8 = iota
	e_u
	e_poc
	e_f
	e_h
)

const (
	rl_nd uint8 = iota
	rl_of
	rl_tf
	rl_w
	rl_u
)

const (
	rc_nd uint8 = iota
	rc_uc
	rc_ur
	rc_c
)

// Environmental

const (
	cdp_nd uint8 = iota
	cdp_n
	cdp_l
	cdp_lm
	cdp_mh
	cdp_h
)

const (
	td_nd uint8 = iota
	td_n
	td_l
	td_m
	td_h
)

const (
	ciar_nd uint8 = iota
	ciar_l
	ciar_m
	ciar_h
)
