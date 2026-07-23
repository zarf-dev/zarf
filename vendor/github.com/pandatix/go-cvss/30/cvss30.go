package gocvss30

import (
	"math"
	"strings"
	"unsafe"
)

// This file is based on https://www.first.org/cvss/v3.0/cvss-v30-specification_v1.9.pdf.

const (
	header = "CVSS:3.0/"
)

// ParseVector parses a given vector string, validates it
// and returns a CVSS30.
func ParseVector(vector string) (*CVSS30, error) {
	// Check header
	if !strings.HasPrefix(vector, header) {
		return nil, ErrInvalidCVSSHeader
	}
	vector = vector[len(header):]

	// Allocate CVSS v3.1 object
	cvss30 := &CVSS30{
		u0: 0,
		u1: 0,
		u2: 0,
		u3: 0,
		u4: 0,
		u5: 0, // last 4 bits are not used
	}

	// Parse vector
	kvm := kvm{}
	start := 0
	l := len(vector)
	for i := 0; i <= l; i++ {
		if i == l || vector[i] == '/' {
			a, v := splitCouple(vector[start:i])
			if err := kvm.Set(a); err != nil {
				return nil, err
			}
			if err := cvss30.Set(a, v); err != nil {
				return nil, err
			}
			start = i + 1
		}
	}

	// Check all base score metrics are defined
	if !kvm.av {
		return nil, &ErrMissing{Abv: "AV"}
	}
	if !kvm.ac {
		return nil, &ErrMissing{Abv: "AC"}
	}
	if !kvm.pr {
		return nil, &ErrMissing{Abv: "PR"}
	}
	if !kvm.ui {
		return nil, &ErrMissing{Abv: "UI"}
	}
	if !kvm.s {
		return nil, &ErrMissing{Abv: "S"}
	}
	if !kvm.c {
		return nil, &ErrMissing{Abv: "C"}
	}
	if !kvm.i {
		return nil, &ErrMissing{Abv: "I"}
	}
	if !kvm.a {
		return nil, &ErrMissing{Abv: "A"}
	}

	return cvss30, nil
}

// splitCouple is more efficient than `strings.Cut` as it is
// specialised on the ':' char.
func splitCouple(couple string) (string, string) {
	for i := 0; i < len(couple); i++ {
		if couple[i] == ':' {
			return couple[:i], couple[i+1:]
		}
	}
	return couple, ""
}

// Vector returns the CVSS v3.1 vector string representation.
func (cvss30 CVSS30) Vector() string {
	l := lenVec(&cvss30)
	b := make([]byte, 0, l)
	b = append(b, header...)

	// Base
	mandatory(&b, "AV:", cvss30.get("AV"))
	mandatory(&b, "/AC:", cvss30.get("AC"))
	mandatory(&b, "/PR:", cvss30.get("PR"))
	mandatory(&b, "/UI:", cvss30.get("UI"))
	mandatory(&b, "/S:", cvss30.get("S"))
	mandatory(&b, "/C:", cvss30.get("C"))
	mandatory(&b, "/I:", cvss30.get("I"))
	mandatory(&b, "/A:", cvss30.get("A"))

	// Temporal
	notMandatory(&b, "/E:", cvss30.get("E"))
	notMandatory(&b, "/RL:", cvss30.get("RL"))
	notMandatory(&b, "/RC:", cvss30.get("RC"))

	// Environmental
	notMandatory(&b, "/CR:", cvss30.get("CR"))
	notMandatory(&b, "/IR:", cvss30.get("IR"))
	notMandatory(&b, "/AR:", cvss30.get("AR"))
	notMandatory(&b, "/MAV:", cvss30.get("MAV"))
	notMandatory(&b, "/MAC:", cvss30.get("MAC"))
	notMandatory(&b, "/MPR:", cvss30.get("MPR"))
	notMandatory(&b, "/MUI:", cvss30.get("MUI"))
	notMandatory(&b, "/MS:", cvss30.get("MS"))
	notMandatory(&b, "/MC:", cvss30.get("MC"))
	notMandatory(&b, "/MI:", cvss30.get("MI"))
	notMandatory(&b, "/MA:", cvss30.get("MA"))

	return *(*string)(unsafe.Pointer(&b))
	// return unsafe.String(&b[0], l)
}

func lenVec(cvss30 *CVSS30) int {
	// Header: constant, so fixed (9)
	// Base:
	// - AV, AC, PR, UI: 4
	// - S, C, I, A: 3
	// - separators: 7
	// Total: 4*4 + 4*3 + 7 = 35
	l := len(header) + 35

	// Temporal:
	// - E: 3
	// - RL, RC: 4
	// - each one adds a separator
	// shortcut for "E" metric
	if (cvss30.u1 & 0b00000111) != 0 {
		l += 4
	}
	// shortcut for "RL" metric
	if (cvss30.u2 & 0b11100000) != 0 {
		l += 5
	}
	// shortcut for "RC" metric
	if (cvss30.u2 & 0b00011000) != 0 {
		l += 5
	}

	// Environmental:
	// - CR, IR, AR, MS, MC, MI, MA: 4
	// - MAV, MAC, MPR, MUI: 5
	// - each one adds a separator
	// shortcut for "CR" metric
	if (cvss30.u2 & 0b00000110) != 0 {
		l += 5
	}
	// shortcut for "IR" metric
	if (cvss30.u2&0b00000001) != 0 || (cvss30.u3&0b10000000) != 0 {
		l += 5
	}
	// shortcut for "AR" metric
	if (cvss30.u3 & 0b01100000) != 0 {
		l += 5
	}
	// shortcut for "MS" metric
	if (cvss30.u4 & 0b00001100) != 0 {
		l += 5
	}
	// shortcut for "MC" metric
	if (cvss30.u4 & 0b00000011) != 0 {
		l += 5
	}
	// shortcut for "MI" metric
	if (cvss30.u5 & 0b11000000) != 0 {
		l += 5
	}
	// shortcut for "MA" metric
	if (cvss30.u5 & 0b00110000) != 0 {
		l += 5
	}
	// shortcut for "MAV" metric
	if (cvss30.u3 & 0b00011100) != 0 {
		l += 6
	}
	// shortcut for "MAC" metric
	if (cvss30.u3 & 0b00000011) != 0 {
		l += 6
	}
	// shortcut for "MPR" metric
	if (cvss30.u4 & 0b11000000) != 0 {
		l += 6
	}
	// shortcut for "MUI" metric
	if (cvss30.u4 & 0b00110000) != 0 {
		l += 6
	}

	return l
}

func mandatory(b *[]byte, pre, v string) {
	*b = append(*b, pre...)
	*b = append(*b, v...)
}

func notMandatory(b *[]byte, pre, v string) {
	if v == "X" {
		return
	}
	mandatory(b, pre, v)
}

// CVSS30 embeds all the metric values defined by the CVSS v3.1
// specification.
type CVSS30 struct {
	u0, u1, u2, u3, u4, u5 uint8
}

// Get returns the value of the given metric abbreviation.
func (cvss30 CVSS30) Get(abv string) (r string, err error) {
	switch abv {
	// Base
	case "AV":
		v := (cvss30.u0 & 0b11000000) >> 6
		switch v {
		case av_n:
			r = "N"
		case av_a:
			r = "A"
		case av_l:
			r = "L"
		case av_p:
			r = "P"
		}
	case "AC":
		v := (cvss30.u0 & 0b00100000) >> 5
		switch v {
		case ac_l:
			r = "L"
		case ac_h:
			r = "H"
		}
	case "PR":
		v := (cvss30.u0 & 0b00011000) >> 3
		switch v {
		case pr_n:
			r = "N"
		case pr_l:
			r = "L"
		case pr_h:
			r = "H"
		}
	case "UI":
		v := (cvss30.u0 & 0b00000100) >> 2
		switch v {
		case ui_n:
			r = "N"
		case ui_r:
			r = "R"
		}
	case "S":
		v := (cvss30.u0 & 0b00000010) >> 1
		switch v {
		case s_u:
			r = "U"
		case s_c:
			r = "C"
		}
	case "C":
		v := ((cvss30.u0 & 0b00000001) << 1) | (cvss30.u1&0b10000000)>>7
		switch v {
		case cia_h:
			r = "H"
		case cia_l:
			r = "L"
		case cia_n:
			r = "N"
		}
	case "I":
		v := (cvss30.u1 & 0b01100000) >> 5
		switch v {
		case cia_h:
			r = "H"
		case cia_l:
			r = "L"
		case cia_n:
			r = "N"
		}
	case "A":
		v := (cvss30.u1 & 0b00011000) >> 3
		switch v {
		case cia_h:
			r = "H"
		case cia_l:
			r = "L"
		case cia_n:
			r = "N"
		}

	// Temporal
	case "E":
		v := cvss30.u1 & 0b00000111
		switch v {
		case e_x:
			r = "X"
		case e_h:
			r = "H"
		case e_f:
			r = "F"
		case e_p:
			r = "P"
		case e_u:
			r = "U"
		}
	case "RL":
		v := (cvss30.u2 & 0b11100000) >> 5
		switch v {
		case rl_x:
			r = "X"
		case rl_u:
			r = "U"
		case rl_w:
			r = "W"
		case rl_t:
			r = "T"
		case rl_o:
			r = "O"
		}
	case "RC":
		v := (cvss30.u2 & 0b00011000) >> 3
		switch v {
		case rc_x:
			r = "X"
		case rc_c:
			r = "C"
		case rc_r:
			r = "R"
		case rc_u:
			r = "U"
		}

	// Environmental
	case "CR":
		v := (cvss30.u2 & 0b00000110) >> 1
		switch v {
		case ciar_x:
			r = "X"
		case ciar_h:
			r = "H"
		case ciar_m:
			r = "M"
		case ciar_l:
			r = "L"
		}
	case "IR":
		v := ((cvss30.u2 & 0b00000001) << 1) | ((cvss30.u3 & 0b10000000) >> 7)
		switch v {
		case ciar_x:
			r = "X"
		case ciar_h:
			r = "H"
		case ciar_m:
			r = "M"
		case ciar_l:
			r = "L"
		}
	case "AR":
		v := (cvss30.u3 & 0b01100000) >> 5
		switch v {
		case ciar_x:
			r = "X"
		case ciar_h:
			r = "H"
		case ciar_m:
			r = "M"
		case ciar_l:
			r = "L"
		}
	case "MAV":
		v := (cvss30.u3 & 0b00011100) >> 2
		switch v {
		case mav_x:
			r = "X"
		case mav_n:
			r = "N"
		case mav_a:
			r = "A"
		case mav_l:
			r = "L"
		case mav_p:
			r = "P"
		}
	case "MAC":
		v := cvss30.u3 & 0b00000011
		switch v {
		case mac_x:
			r = "X"
		case mac_l:
			r = "L"
		case mac_h:
			r = "H"
		}
	case "MPR":
		v := (cvss30.u4 & 0b11000000) >> 6
		switch v {
		case mpr_x:
			r = "X"
		case mpr_n:
			r = "N"
		case mpr_l:
			r = "L"
		case mpr_h:
			r = "H"
		}
	case "MUI":
		v := (cvss30.u4 & 0b00110000) >> 4
		switch v {
		case mui_x:
			r = "X"
		case mui_n:
			r = "N"
		case mui_r:
			r = "R"
		}
	case "MS":
		v := (cvss30.u4 & 0b00001100) >> 2
		switch v {
		case ms_x:
			r = "X"
		case ms_u:
			r = "U"
		case ms_c:
			r = "C"
		}
	case "MC":
		v := cvss30.u4 & 0b00000011
		switch v {
		case mcia_x:
			r = "X"
		case mcia_n:
			r = "N"
		case mcia_l:
			r = "L"
		case mcia_h:
			r = "H"
		}
	case "MI":
		v := (cvss30.u5 & 0b11000000) >> 6
		switch v {
		case mcia_x:
			r = "X"
		case mcia_n:
			r = "N"
		case mcia_l:
			r = "L"
		case mcia_h:
			r = "H"
		}
	case "MA":
		v := (cvss30.u5 & 0b00110000) >> 4
		switch v {
		case mcia_x:
			r = "X"
		case mcia_n:
			r = "N"
		case mcia_l:
			r = "L"
		case mcia_h:
			r = "H"
		}
	default:
		err = &ErrInvalidMetric{Abv: abv}
	}
	return
}

// Set sets the value of the given metric abbreviation.
func (cvss30 *CVSS30) Set(abv string, value string) error {
	switch abv {
	// Base
	case "AV":
		v, err := validate(value, []string{"N", "A", "L", "P"})
		if err != nil {
			return err
		}
		cvss30.u0 = (cvss30.u0 & 0b00111111) | (v << 6)
	case "AC":
		v, err := validate(value, []string{"L", "H"})
		if err != nil {
			return err
		}
		cvss30.u0 = (cvss30.u0 & 0b11011111) | (v << 5)
	case "PR":
		v, err := validate(value, []string{"N", "L", "H"})
		if err != nil {
			return err
		}
		cvss30.u0 = (cvss30.u0 & 0b11100111) | (v << 3)
	case "UI":
		v, err := validate(value, []string{"N", "R"})
		if err != nil {
			return err
		}
		cvss30.u0 = (cvss30.u0 & 0b11111011) | (v << 2)
	case "S":
		v, err := validate(value, []string{"U", "C"})
		if err != nil {
			return err
		}
		cvss30.u0 = (cvss30.u0 & 0b11111101) | (v << 1)
	case "C":
		v, err := validate(value, []string{"H", "L", "N"})
		if err != nil {
			return err
		}
		cvss30.u0 = (cvss30.u0 & 0b11111110) | ((v & 0b10) >> 1)
		cvss30.u1 = (cvss30.u1 & 0b01111111) | ((v & 0b01) << 7)
	case "I":
		v, err := validate(value, []string{"H", "L", "N"})
		if err != nil {
			return err
		}
		cvss30.u1 = (cvss30.u1 & 0b10011111) | (v << 5)
	case "A":
		v, err := validate(value, []string{"H", "L", "N"})
		if err != nil {
			return err
		}
		cvss30.u1 = (cvss30.u1 & 0b11100111) | (v << 3)

	// Temporal
	case "E":
		v, err := validate(value, []string{"X", "H", "F", "P", "U"})
		if err != nil {
			return err
		}
		cvss30.u1 = (cvss30.u1 & 0b11111000) | v
	case "RL":
		v, err := validate(value, []string{"X", "U", "W", "T", "O"})
		if err != nil {
			return err
		}
		cvss30.u2 = (cvss30.u2 & 0b00011111) | (v << 5)
	case "RC":
		v, err := validate(value, []string{"X", "C", "R", "U"})
		if err != nil {
			return err
		}
		cvss30.u2 = (cvss30.u2 & 0b11100111) | (v << 3)

	// Environmental
	case "CR":
		v, err := validate(value, []string{"X", "H", "M", "L"})
		if err != nil {
			return err
		}
		cvss30.u2 = (cvss30.u2 & 0b11111001) | (v << 1)
	case "IR":
		v, err := validate(value, []string{"X", "H", "M", "L"})
		if err != nil {
			return err
		}
		cvss30.u2 = (cvss30.u2 & 0b11111110) | ((v & 0b10) >> 1)
		cvss30.u3 = (cvss30.u3 & 0b01111111) | ((v & 0b01) << 7)
	case "AR":
		v, err := validate(value, []string{"X", "H", "M", "L"})
		if err != nil {
			return err
		}
		cvss30.u3 = (cvss30.u3 & 0b10011111) | (v << 5)
	case "MAV":
		v, err := validate(value, []string{"X", "N", "A", "L", "P"})
		if err != nil {
			return err
		}
		cvss30.u3 = (cvss30.u3 & 0b11100011) | (v << 2)
	case "MAC":
		v, err := validate(value, []string{"X", "L", "H"})
		if err != nil {
			return err
		}
		cvss30.u3 = (cvss30.u3 & 0b11111100) | v
	case "MPR":
		v, err := validate(value, []string{"X", "N", "L", "H"})
		if err != nil {
			return err
		}
		cvss30.u4 = (cvss30.u4 & 0b00111111) | (v << 6)
	case "MUI":
		v, err := validate(value, []string{"X", "N", "R"})
		if err != nil {
			return err
		}
		cvss30.u4 = (cvss30.u4 & 0b11001111) | (v << 4)
	case "MS":
		v, err := validate(value, []string{"X", "U", "C"})
		if err != nil {
			return err
		}
		cvss30.u4 = (cvss30.u4 & 0b11110011) | (v << 2)
	case "MC":
		v, err := validate(value, []string{"X", "H", "L", "N"})
		if err != nil {
			return err
		}
		cvss30.u4 = (cvss30.u4 & 0b11111100) | v
	case "MI":
		v, err := validate(value, []string{"X", "H", "L", "N"})
		if err != nil {
			return err
		}
		cvss30.u5 = (cvss30.u5 & 0b00111111) | (v << 6)
	case "MA":
		v, err := validate(value, []string{"X", "H", "L", "N"})
		if err != nil {
			return err
		}
		cvss30.u5 = (cvss30.u5 & 0b11000000) | (v << 4)
	default:
		return &ErrInvalidMetric{Abv: abv}
	}
	return nil
}

// validate returns the index of value in enabled if matches.
// enabled values have to match the values.go constants order.
func validate(value string, enabled []string) (i uint8, err error) {
	// Check is valid
	for _, enbl := range enabled {
		if value == enbl {
			return i, nil
		}
		i++
	}
	return 0, ErrInvalidMetricValue
}

// get is used for internal purposes only.
func (cvss30 CVSS30) get(abv string) string {
	str, _ := cvss30.Get(abv)
	return str
}

// BaseScore returns the CVSS v3.0's base score.
func (cvss30 CVSS30) BaseScore() float64 {
	impact := cvss30.Impact()
	exploitability := cvss30.Exploitability()
	if impact <= 0 {
		return 0
	}
	// shortcut to avoid get("S") -> improve performances by ~40%
	if cvss30.u0&0b00000010 == 0 {
		return roundup(math.Min(impact+exploitability, 10))
	}
	return roundup(math.Min(1.08*(impact+exploitability), 10))
}

func (cvss30 CVSS30) Impact() float64 {
	// directly lookup for variables without get -> improve performances by ~20%
	c := cia(((cvss30.u0 & 0b00000001) << 1) | (cvss30.u1&0b10000000)>>7)
	i := cia((cvss30.u1 & 0b01100000) >> 5)
	a := cia((cvss30.u1 & 0b00011000) >> 3)
	iss := 1 - ((1 - c) * (1 - i) * (1 - a))
	// shortcut to avoid get("S") -> improve performances by ~40%
	if cvss30.u0&0b00000010 == 0 {
		return 6.42 * iss
	}
	return 7.52*(iss-0.029) - 3.25*pow15(iss-0.02)
}

func (cvss30 CVSS30) Exploitability() float64 {
	// directly lookup for variables without get -> improve performances by ~20%
	av := attackVector((cvss30.u0 & 0b11000000) >> 6)
	ac := attackComplexity((cvss30.u0 & 0b00100000) >> 5)
	pr := privilegesRequired((cvss30.u0&0b00011000)>>3, (cvss30.u0&0b00000010)>>1)
	ui := userInteraction((cvss30.u0 & 0b00000100) >> 2)
	return 8.22 * av * ac * pr * ui
}

// TemporalScore returns the CVSS v3.1's temporal score.
func (cvss30 CVSS30) TemporalScore() float64 {
	e := exploitCodeMaturity(cvss30.u1 & 0b00000111)
	rl := remediationLevel((cvss30.u2 & 0b11100000) >> 5)
	rc := reportConfidence((cvss30.u2 & 0b00011000) >> 3)
	return roundup(cvss30.BaseScore() * e * rl * rc)
}

// EnvironmentalScore returns the CVSS v3.1's environmental score.
func (cvss30 CVSS30) EnvironmentalScore() float64 {
	// Choose which to use (use base if modified is not defined).
	// It is based on first.org online calculator's source code,
	// while it is not explicit in the specification which value
	// to use.
	mav := mod((cvss30.u0&0b11000000)>>6, (cvss30.u3&0b00011100)>>2)
	mac := mod((cvss30.u0&0b00100000)>>5, cvss30.u3&0b00000011)
	mpr := mod((cvss30.u0&0b00011000)>>3, (cvss30.u4&0b11000000)>>6)
	mui := mod((cvss30.u0&0b00000100)>>2, (cvss30.u4&0b00110000)>>4)
	ms := mod((cvss30.u0&0b00000010)>>1, (cvss30.u4&0b00001100)>>2)
	mc := mod(((cvss30.u0&0b00000001)<<1)|((cvss30.u1&0b10000000)>>7), cvss30.u4&0b00000011)
	mi := mod((cvss30.u1&0b01100000)>>5, (cvss30.u5&0b11000000)>>6)
	ma := mod((cvss30.u1&0b00011000)>>3, (cvss30.u5&0b00110000)>>4)

	cr := ciar((cvss30.u2 & 0b00000110) >> 1)
	ir := ciar(((cvss30.u2 & 0b00000001) << 1) | ((cvss30.u3 & 0b10000000) >> 7))
	ar := ciar((cvss30.u3 & 0b01100000) >> 5)
	e := exploitCodeMaturity(cvss30.u1 & 0b00000111)
	rl := remediationLevel((cvss30.u2 & 0b11100000) >> 5)
	rc := reportConfidence((cvss30.u2 & 0b00011000) >> 3)
	miss := math.Min(1-(1-cr*cia(mc))*(1-ir*cia(mi))*(1-ar*cia(ma)), 0.915)
	var modifiedImpact float64
	if ms == s_u {
		modifiedImpact = 6.42 * miss
	} else {
		modifiedImpact = 7.52*(miss-0.029) - 3.25*pow15(miss-0.02)
	}
	modifiedExploitability := 8.22 * attackVector(mav) * attackComplexity(mac) * privilegesRequired(mpr, ms) * userInteraction(mui)
	if modifiedImpact <= 0 {
		return 0
	}
	if ms == s_u {
		return roundup(roundup(math.Min(modifiedImpact+modifiedExploitability, 10)) * e * rl * rc)
	}
	r := math.Min(1.08*(modifiedImpact+modifiedExploitability), 10)
	return roundup(roundup(r) * e * rl * rc)
}

// Rating returns the verbose for a given rating.
// It does not check wether the number of decimal is valid,
// as it can differ due to binary imprecisions, and such
// behaviour is not enforced by the specification.
func Rating(score float64) (string, error) {
	if score < 0.0 || score > 10.0 {
		return "", ErrOutOfBoundsScore
	}
	if score >= 9.0 {
		return "CRITICAL", nil
	}
	if score >= 7.0 {
		return "HIGH", nil
	}
	if score >= 4.0 {
		return "MEDIUM", nil
	}
	if score >= 0.1 {
		return "LOW", nil
	}
	return "NONE", nil
}

// Helpers to compute CVSS v3.1 scores

func attackVector(v uint8) float64 {
	switch v {
	case av_n:
		return 0.85
	case av_a:
		return 0.62
	case av_l:
		return 0.55
	case av_p:
		return 0.2
	default:
		panic(ErrInvalidMetricValue)
	}
}

func attackComplexity(v uint8) float64 {
	switch v {
	case ac_l:
		return 0.77
	case ac_h:
		return 0.44
	default:
		panic(ErrInvalidMetricValue)
	}
}

func privilegesRequired(v, scope uint8) float64 {
	switch v {
	case pr_n:
		return 0.85
	case pr_l:
		if scope == s_c {
			return 0.68
		}
		return 0.62
	case pr_h:
		if scope == s_c {
			return 0.5
		}
		return 0.27
	default:
		panic(ErrInvalidMetricValue)
	}
}

func userInteraction(v uint8) float64 {
	switch v {
	case ui_n:
		return 0.85
	case ui_r:
		return 0.62
	default:
		panic(ErrInvalidMetricValue)
	}
}

func cia(v uint8) float64 {
	switch v {
	case cia_h:
		return 0.56
	case cia_l:
		return 0.22
	case cia_n:
		return 0
	default:
		panic(ErrInvalidMetricValue)
	}
}

func exploitCodeMaturity(v uint8) float64 {
	switch v {
	case e_x, e_h:
		return 1
	case e_f:
		return 0.97
	case e_p:
		return 0.94
	case e_u:
		return 0.91
	default:
		panic(ErrInvalidMetricValue)
	}
}

func remediationLevel(v uint8) float64 {
	switch v {
	case rl_x, rl_u:
		return 1
	case rl_w:
		return 0.97
	case rl_t:
		return 0.96
	case rl_o:
		return 0.95
	default:
		panic(ErrInvalidMetricValue)
	}
}

func reportConfidence(v uint8) float64 {
	switch v {
	case rc_x, rc_c:
		return 1
	case rc_r:
		return 0.96
	case rc_u:
		return 0.92
	default:
		panic(ErrInvalidMetricValue)
	}
}

func ciar(v uint8) float64 {
	switch v {
	case ciar_x, ciar_m:
		return 1
	case ciar_h:
		return 1.5
	case ciar_l:
		return 0.5
	default:
		panic(ErrInvalidMetricValue)
	}
}

func roundup(x float64) float64 {
	bx := math.RoundToEven(x * 100000)
	if int(bx)%10000 == 0 {
		return bx / 100000.0
	}
	return (math.Floor(bx/10000) + 1) / 10.0
}

func pow15(f float64) float64 {
	f2 := f * f
	f3 := f2 * f
	f5 := f2 * f3
	return f5 * f5 * f5
}

func mod(base, modified uint8) uint8 {
	// If "modified" is different of 0, it is different of "X"
	// => shift to one before (skip X index)
	if modified != 0 {
		return modified - 1
	}
	return base
}

// kvm stands for Key-Value Map, and is used to make sure each
// metric is defined only once, as documented by the CVSS v3.1
// specification document, section 6 "Vector String" paragraph 3.
// Using this avoids a map that escapes to heap for each call of
// ParseVector, as its size is known and wont evolve.
type kvm struct {
	// base metrics
	av, ac, pr, ui, s, c, i, a bool
	// temporal metrics
	e, rl, rc bool
	// environmental metrics
	cr, ir, ar, mav, mac, mpr, mui, ms, mc, mi, ma bool
}

func (kvm *kvm) Set(abv string) error {
	var dst *bool
	switch abv {
	case "AV":
		dst = &kvm.av
	case "AC":
		dst = &kvm.ac
	case "PR":
		dst = &kvm.pr
	case "UI":
		dst = &kvm.ui
	case "S":
		dst = &kvm.s
	case "C":
		dst = &kvm.c
	case "I":
		dst = &kvm.i
	case "A":
		dst = &kvm.a
	case "E":
		dst = &kvm.e
	case "RL":
		dst = &kvm.rl
	case "RC":
		dst = &kvm.rc
	case "CR":
		dst = &kvm.cr
	case "IR":
		dst = &kvm.ir
	case "AR":
		dst = &kvm.ar
	case "MAV":
		dst = &kvm.mav
	case "MAC":
		dst = &kvm.mac
	case "MPR":
		dst = &kvm.mpr
	case "MUI":
		dst = &kvm.mui
	case "MS":
		dst = &kvm.ms
	case "MC":
		dst = &kvm.mc
	case "MI":
		dst = &kvm.mi
	case "MA":
		dst = &kvm.ma
	default:
		return &ErrInvalidMetric{Abv: abv}
	}
	if *dst {
		return &ErrDefinedN{Abv: abv}
	}
	*dst = true
	return nil
}
