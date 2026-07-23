package gocvss40

// The following values are the enumerations of each metric possible values.
// Do not change their order as it is vital to the implementation.
//
// Those are used to get a highly memory-performant implementation.
// The 9 bytes are used as follows, using the uint8 type.
//
//    u0       u1       u2       u3       u4       u5       u6       u7       u8
// /------\ /------\ /------\ /------\ /------\ /------\ /------\ /------\ /------\
// ........ ........ ........ ........ ........ ........ ........ ........ ........
// \/||\/\/ \/\/\/\/ \/\/\/\/ \/\/\_/\_/\/\/\/\_/\/\/\/\__/\_/\/\_/\/\/\/\__/
// AV|AT|UI VC |VI | VA | E | IR |MAV | MAT|MUI| MVI|MSC | MSA S AU | V |  U
//  AC PR     SC  SI   SA  CR   AR   MAC  MPR MVC  MVA  MSI         R  RE

// Base

const (
	av_n uint8 = iota
	av_a
	av_l
	av_p
)

const (
	ac_h uint8 = iota
	ac_l
)

const (
	at_n uint8 = iota
	at_p
)

const (
	pr_h uint8 = iota
	pr_l
	pr_n
)

const (
	ui_n uint8 = iota
	ui_p
	ui_a
)

const (
	vscia_h uint8 = iota
	vscia_l
	vscia_n
	// vscia_s is only used during severity distances computation, it
	// is not a valid value for SI/SA and not even a valid value for MSC.
	vscia_s
)

// Threat

const (
	e_x uint8 = iota
	e_a
	e_p
	e_u
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
	mac_h
	mac_l
)

const (
	mat_x uint8 = iota
	mat_n
	mat_p
)

const (
	mpr_x uint8 = iota
	mpr_h
	mpr_l
	mpr_n
)

const (
	mui_x uint8 = iota
	mui_n
	mui_p
	mui_a
)

const (
	mvcia_x uint8 = iota
	mvcia_h
	mvcia_l
	mvcia_n
)

const (
	msc_x uint8 = iota
	msc_h
	msc_l
	msc_n
)

const (
	msia_x uint8 = iota
	msia_h
	msia_l
	msia_n
	msia_s
)

// Supplemental

const (
	s_x uint8 = iota
	s_n
	s_p
)

const (
	au_x uint8 = iota
	au_n
	au_y
)

const (
	r_x uint8 = iota
	r_a
	r_u
	r_i
)

const (
	v_x uint8 = iota
	v_d
	v_c
)

const (
	re_x uint8 = iota
	re_l
	re_m
	re_h
)

const (
	u_x uint8 = iota
	u_clear
	u_green
	u_amber
	u_red
)

// The following values are used to dance in memory :)

const (
	av uint8 = iota
	ac
	at
	pr
	ui
	vc
	vi
	va
	sc
	si
	sa
	e
	cr
	ir
	ar
)
