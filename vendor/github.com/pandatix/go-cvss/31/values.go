package gocvss31

// The following values are the enumerations of each metric possible values.
// Do not change their order as it is vital to the implementation.
//
// Those are used to get a highly memory-performant implementation.
// The 6 bytes are used as follows, using the uint8 type.
//
//    u0       u1       u2       u3       u4       u5
// /------\ /------\ /------\ /------\ /------\ /------\
// ........ ........ ........ ........ ........ ........
// \/|\/||\_/\/\/\_/ \_/\/\/\_/\/\_/\/ \/\/\/\/ \/\/
// AV|PR|S C  I A E   RL |CR IR |MAV |MPR |MS | MI |
//   AC UI              RC     AR   MAC  MUI MC   MA

// Base

const (
	av_n uint8 = iota
	av_a
	av_l
	av_p
)

const (
	ac_l uint8 = iota
	ac_h
)

const (
	pr_n uint8 = iota
	pr_l
	pr_h
)

const (
	ui_n uint8 = iota
	ui_r
)

const (
	s_u uint8 = iota
	s_c
)

const (
	cia_h uint8 = iota
	cia_l
	cia_n
)

// Temporal

const (
	e_x uint8 = iota
	e_h
	e_f
	e_p
	e_u
)

const (
	rl_x uint8 = iota
	rl_u
	rl_w
	rl_t
	rl_o
)

const (
	rc_x uint8 = iota
	rc_c
	rc_r
	rc_u
)

// Environmental

const (
	ciar_x uint8 = iota
	ciar_h
	ciar_m
	ciar_l
)

const (
	mav_x uint8 = iota
	mav_n
	mav_a
	mav_l
	mav_p
)

const (
	mac_x uint8 = iota
	mac_l
	mac_h
)

const (
	mpr_x uint8 = iota
	mpr_n
	mpr_l
	mpr_h
)

const (
	mui_x uint8 = iota
	mui_n
	mui_r
)

const (
	ms_x uint8 = iota
	ms_u
	ms_c
)

const (
	mcia_x uint8 = iota
	mcia_h
	mcia_l
	mcia_n
)
