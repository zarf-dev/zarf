package gocvss20

import (
	"math"
	"strings"
	"sync"
	"unsafe"
)

var order = [][]string{
	{"AV", "AC", "Au", "C", "I", "A"}, // Base metrics
	{"E", "RL", "RC"},                 // Temporal metrics
	{"CDP", "TD", "CR", "IR", "AR"},   // Environmental metrics
}

// ParseVector parses a CVSS v2.0 vector.
func ParseVector(vector string) (*CVSS20, error) {
	// Split parts
	partsPtr := splitPool.Get()
	defer splitPool.Put(partsPtr)
	pts := partsPtr.([]string)
	ei := split(pts, vector)
	pts = pts[:ei+1]

	// Work on each CVSS part
	cvss20 := &CVSS20{
		u0: 0,
		u1: 0,
		u2: 0,
		u3: 0,
	}

	slci := 0
	i := 0
	for _, pt := range pts {
		abv, v, _ := strings.Cut(pt, ":")
		tgt := ""
		switch slci {
		case 0, 2:
			tgt = order[slci][i]
		case 1:
			tgt = order[1][i]
			if i == 0 && tgt != abv {
				slci++
				tgt = order[2][0]
			}
		default:
			return nil, ErrInvalidMetricValue
		}
		if abv != tgt {
			return nil, ErrInvalidMetricOrder
		}

		if err := cvss20.Set(abv, v); err != nil {
			return nil, err
		}

		// Go to next element in slice, or next slice if fully consumed
		i++
		if i == len(order[slci]) {
			slci++
			i = 0
		}
	}
	// Check whole last metric group is specified in vector (=> i == 0)
	if i != 0 {
		return nil, ErrTooShortVector
	}

	return cvss20, nil
}

var splitPool = sync.Pool{
	New: func() any {
		return make([]string, 14)
	},
}

func split(dst []string, vector string) int {
	start := 0
	curr := 0
	l := len(vector)
	i := 0
	for ; i < l; i++ {
		if vector[i] == '/' {
			dst[curr] = vector[start:i]

			start = i + 1
			curr++

			if curr == 13 {
				break
			}
		}
	}
	dst[curr] = vector[start:]
	return curr
}

func (cvss20 CVSS20) Vector() string {
	l := lenVec(&cvss20)
	b := make([]byte, 0, l)

	// Base
	app(&b, "AV:", cvss20.get("AV"))
	app(&b, "/AC:", cvss20.get("AC"))
	app(&b, "/Au:", cvss20.get("Au"))
	app(&b, "/C:", cvss20.get("C"))
	app(&b, "/I:", cvss20.get("I"))
	app(&b, "/A:", cvss20.get("A"))

	// Temporal
	e, rl, rc := cvss20.get("E"), cvss20.get("RL"), cvss20.get("RC")
	if e != "ND" || rl != "ND" || rc != "ND" {
		app(&b, "/E:", e)
		app(&b, "/RL:", rl)
		app(&b, "/RC:", rc)
	}

	// Environmental
	cdp, td, cr, ir, ar := cvss20.get("CDP"), cvss20.get("TD"), cvss20.get("CR"), cvss20.get("IR"), cvss20.get("AR")
	if cdp != "ND" || td != "ND" || cr != "ND" || ir != "ND" || ar != "ND" {
		app(&b, "/CDP:", cdp)
		app(&b, "/TD:", td)
		app(&b, "/CR:", cr)
		app(&b, "/IR:", ir)
		app(&b, "/AR:", ar)
	}

	return *(*string)(unsafe.Pointer(&b))
	// return unsafe.String(&b[0], l)
}

func lenVec(cvss20 *CVSS20) int {
	// Base:
	// - AV, AC, Au: 4
	// - C, I, A: 3
	// - separators: 5
	// Total: 3*4 + 3*3 + 5 = 26
	l := 26

	// Temporal:
	// - E: 2 + len(v)
	// - RL: 3 + len(v)
	// - RC: 3 + len(v)
	// - separators: 3
	// Total: 11 + 3*len(v)
	e, rl, rc := cvss20.get("E"), cvss20.get("RL"), cvss20.get("RC")
	if e != "ND" || rl != "ND" || rc != "ND" {
		l += 11 + len(e) + len(rl) + len(rc)
	}

	// Environmental:
	// - CDP: 4 + len(v)
	// - TD: 3 + len(v)
	// - CR, IR, AR: 3 + len(v)
	// - separators: 5
	// Total: 21 + 5*len(v)
	cdp, td, cr, ir, ar := cvss20.get("CDP"), cvss20.get("TD"), cvss20.get("CR"), cvss20.get("IR"), cvss20.get("AR")
	if cdp != "ND" || td != "ND" || cr != "ND" || ir != "ND" || ar != "ND" {
		l += 21 + len(cdp) + len(td) + len(cr) + len(ir) + len(ar)
	}

	return l
}

func app(b *[]byte, pre, v string) {
	*b = append(*b, pre...)
	*b = append(*b, v...)
}

// CVSS20 embeds all the metric values defined by the CVSS v2.0
// rev2 specification.
type CVSS20 struct {
	u0, u1, u2, u3 uint8
}

func (cvss20 CVSS20) Get(abv string) (r string, err error) {
	switch abv {
	// Base
	case "AV":
		v := (cvss20.u0 & 0b11000000) >> 6
		switch v {
		case av_l:
			r = "L"
		case av_a:
			r = "A"
		case av_n:
			r = "N"
		}
	case "AC":
		v := (cvss20.u0 & 0b00110000) >> 4
		switch v {
		case ac_l:
			r = "L"
		case ac_m:
			r = "M"
		case ac_h:
			r = "H"
		}
	case "Au":
		v := (cvss20.u0 & 0b00001100) >> 2
		switch v {
		case au_m:
			r = "M"
		case au_s:
			r = "S"
		case au_n:
			r = "N"
		}
	case "C":
		v := cvss20.u0 & 0b00000011
		switch v {
		case cia_n:
			r = "N"
		case cia_p:
			r = "P"
		case cia_c:
			r = "C"
		}
	case "I":
		v := (cvss20.u1 & 0b11000000) >> 6
		switch v {
		case cia_n:
			r = "N"
		case cia_p:
			r = "P"
		case cia_c:
			r = "C"
		}
	case "A":
		v := (cvss20.u1 & 0b00110000) >> 4
		switch v {
		case cia_n:
			r = "N"
		case cia_p:
			r = "P"
		case cia_c:
			r = "C"
		}

	// Temporal
	case "E":
		v := (cvss20.u1 & 0b00001110) >> 1
		switch v {
		case e_nd:
			r = "ND"
		case e_u:
			r = "U"
		case e_poc:
			r = "POC"
		case e_f:
			r = "F"
		case e_h:
			r = "H"
		}
	case "RL":
		v := ((cvss20.u1 & 0b00000001) << 2) | ((cvss20.u2 & 0b11000000) >> 6)
		switch v {
		case rl_nd:
			r = "ND"
		case rl_of:
			r = "OF"
		case rl_tf:
			r = "TF"
		case rl_w:
			r = "W"
		case rl_u:
			r = "U"
		}
	case "RC":
		v := (cvss20.u2 & 0b00110000) >> 4
		switch v {
		case rc_nd:
			r = "ND"
		case rc_uc:
			r = "UC"
		case rc_ur:
			r = "UR"
		case rc_c:
			r = "C"
		}

	// Environmental
	case "CDP":
		v := (cvss20.u2 & 0b00001110) >> 1
		switch v {
		case cdp_nd:
			r = "ND"
		case cdp_n:
			r = "N"
		case cdp_l:
			r = "L"
		case cdp_lm:
			r = "LM"
		case cdp_mh:
			r = "MH"
		case cdp_h:
			r = "H"
		}
	case "TD":
		v := ((cvss20.u2 & 0b00000001) << 2) | ((cvss20.u3 & 0b11000000) >> 6)
		switch v {
		case td_nd:
			r = "ND"
		case td_n:
			r = "N"
		case td_l:
			r = "L"
		case td_m:
			r = "M"
		case td_h:
			r = "H"
		}
	case "CR":
		v := (cvss20.u3 & 0b00110000) >> 4
		switch v {
		case ciar_nd:
			r = "ND"
		case ciar_l:
			r = "L"
		case ciar_m:
			r = "M"
		case ciar_h:
			r = "H"
		}
	case "IR":
		v := (cvss20.u3 & 0b00001100) >> 2
		switch v {
		case ciar_nd:
			r = "ND"
		case ciar_l:
			r = "L"
		case ciar_m:
			r = "M"
		case ciar_h:
			r = "H"
		}
	case "AR":
		v := cvss20.u3 & 0b00000011
		switch v {
		case ciar_nd:
			r = "ND"
		case ciar_l:
			r = "L"
		case ciar_m:
			r = "M"
		case ciar_h:
			r = "H"
		}
	default:
		return "", &ErrInvalidMetric{Abv: abv}
	}
	return
}

// get is used for internal purposes only.
func (cvss20 CVSS20) get(abv string) string {
	str, err := cvss20.Get(abv)
	if err != nil {
		panic(err)
	}
	return str
}

func (cvss20 *CVSS20) Set(abv string, value string) error {
	switch abv {
	// Base
	case "AV":
		v, err := validate(value, []string{"L", "A", "N"})
		if err != nil {
			return err
		}
		cvss20.u0 = (cvss20.u0 & 0b00111111) | (v << 6)
	case "AC":
		v, err := validate(value, []string{"L", "M", "H"})
		if err != nil {
			return err
		}
		cvss20.u0 = (cvss20.u0 & 0b11001111) | (v << 4)
	case "Au":
		v, err := validate(value, []string{"M", "S", "N"})
		if err != nil {
			return err
		}
		cvss20.u0 = (cvss20.u0 & 0b11110011) | (v << 2)
	case "C":
		v, err := validate(value, []string{"N", "P", "C"})
		if err != nil {
			return err
		}
		cvss20.u0 = (cvss20.u0 & 0b11111100) | v
	case "I":
		v, err := validate(value, []string{"N", "P", "C"})
		if err != nil {
			return err
		}
		cvss20.u1 = (cvss20.u1 & 0b00111111) | (v << 6)
	case "A":
		v, err := validate(value, []string{"N", "P", "C"})
		if err != nil {
			return err
		}
		cvss20.u1 = (cvss20.u1 & 0b11001111) | (v << 4)

	// Temporal
	case "E":
		v, err := validate(value, []string{"ND", "U", "POC", "F", "H"})
		if err != nil {
			return err
		}
		cvss20.u1 = (cvss20.u1 & 0b11110001) | (v << 1)
	case "RL":
		v, err := validate(value, []string{"ND", "OF", "TF", "W", "U"})
		if err != nil {
			return err
		}
		cvss20.u1 = (cvss20.u1 & 0b11111110) | ((v & 0b100) >> 2)
		cvss20.u2 = (cvss20.u2 & 0b00111111) | ((v & 0b011) << 6)
	case "RC":
		v, err := validate(value, []string{"ND", "UC", "UR", "C"})
		if err != nil {
			return err
		}
		cvss20.u2 = (cvss20.u2 & 0b11001111) | (v << 4)

	// Environmental
	case "CDP":
		v, err := validate(value, []string{"ND", "N", "L", "LM", "MH", "H"})
		if err != nil {
			return err
		}
		cvss20.u2 = (cvss20.u2 & 0b11110001) | (v << 1)
	case "TD":
		v, err := validate(value, []string{"ND", "N", "L", "M", "H"})
		if err != nil {
			return err
		}
		cvss20.u2 = (cvss20.u2 & 0b11111110) | ((v & 0b100) >> 2)
		cvss20.u3 = (cvss20.u3 & 0b00111111) | ((v & 0b011) << 6)
	case "CR":
		v, err := validate(value, []string{"ND", "L", "M", "H"})
		if err != nil {
			return err
		}
		cvss20.u3 = (cvss20.u3 & 0b11001111) | (v << 4)
	case "IR":
		v, err := validate(value, []string{"ND", "L", "M", "H"})
		if err != nil {
			return err
		}
		cvss20.u3 = (cvss20.u3 & 0b11110011) | (v << 2)
	case "AR":
		v, err := validate(value, []string{"ND", "L", "M", "H"})
		if err != nil {
			return err
		}
		cvss20.u3 = (cvss20.u3 & 0b11111100) | v
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

// BaseScore returns the CVSS v2.0's base score.
func (cvss20 CVSS20) BaseScore() float64 {
	impact := cvss20.Impact()
	fimpact := 0.0
	if impact != 0 {
		fimpact = 1.176
	}
	exploitability := cvss20.Exploitability()
	return roundTo1Decimal(((0.6 * impact) + (0.4 * exploitability) - 1.5) * fimpact)
}

func (cvss20 CVSS20) Impact() float64 {
	c := cia(cvss20.u0 & 0b00000011)
	i := cia((cvss20.u1 & 0b11000000) >> 6)
	a := cia((cvss20.u1 & 0b00110000) >> 4)
	return 10.41 * (1 - (1-c)*(1-i)*(1-a))
}

func (cvss20 CVSS20) Exploitability() float64 {
	av := accessVector((cvss20.u0 & 0b11000000) >> 6)
	ac := accessComplexity((cvss20.u0 & 0b00110000) >> 4)
	au := authentication((cvss20.u0 & 0b00001100) >> 2)
	return 20 * av * ac * au
}

// TemporalScore returns the CVSS v2.0's temporal score.
func (cvss20 CVSS20) TemporalScore() float64 {
	e := exploitability((cvss20.u1 & 0b00001110) >> 1)
	rl := remediationLevel(((cvss20.u1 & 0b00000001) << 2) | ((cvss20.u2 & 0b11000000) >> 6))
	rc := reportConfidence((cvss20.u2 & 0b00110000) >> 4)
	return roundTo1Decimal(cvss20.BaseScore() * e * rl * rc)
}

// EnvironmentalScore returns the CVSS v2.0's environmental score.
func (cvss20 CVSS20) EnvironmentalScore() float64 {
	c := cia(cvss20.u0 & 0b00000011)
	i := cia((cvss20.u1 & 0b11000000) >> 6)
	a := cia((cvss20.u1 & 0b00110000) >> 4)
	cr := ciar((cvss20.u3 & 0b00110000) >> 4)
	ir := ciar((cvss20.u3 & 0b00001100) >> 2)
	ar := ciar(cvss20.u3 & 0b00000011)
	adjustedImpact := math.Min(10, 10.41*(1-(1-c*cr)*(1-i*ir)*(1-a*ar)))
	fimpactBase := 0.0
	if adjustedImpact != 0 {
		fimpactBase = 1.176
	}
	expltBase := cvss20.Exploitability()
	e := exploitability((cvss20.u1 & 0b00001110) >> 1)
	rl := remediationLevel(((cvss20.u1 & 0b00000001) << 2) | ((cvss20.u2 & 0b11000000) >> 6))
	rc := reportConfidence((cvss20.u2 & 0b00110000) >> 4)
	recBase := roundTo1Decimal(((0.6 * adjustedImpact) + (0.4 * expltBase) - 1.5) * fimpactBase)
	adjustedTemporal := roundTo1Decimal(recBase * e * rl * rc)
	cdp := collateralDamagePotential((cvss20.u2 & 0b00001110) >> 1)
	td := targetDistribution(((cvss20.u2 & 0b00000001) << 2) | ((cvss20.u3 & 0b11000000) >> 6))
	return roundTo1Decimal((adjustedTemporal + (10-adjustedTemporal)*cdp) * td)
}

// Helpers to compute CVSS v2.0 scores.

func accessVector(v uint8) float64 {
	switch v {
	case av_l:
		return 0.395
	case av_a:
		return 0.646
	case av_n:
		return 1.0
	default:
		panic(ErrInvalidMetricValue)
	}
}

func accessComplexity(v uint8) float64 {
	switch v {
	case ac_h:
		return 0.35
	case ac_m:
		return 0.61
	case ac_l:
		return 0.71
	default:
		panic(ErrInvalidMetricValue)
	}
}

func authentication(v uint8) float64 {
	switch v {
	case au_m:
		return 0.45
	case au_s:
		return 0.56
	case au_n:
		return 0.704
	default:
		panic(ErrInvalidMetricValue)
	}
}

func cia(v uint8) float64 {
	switch v {
	case cia_n:
		return 0.0
	case cia_p:
		return 0.275
	case cia_c:
		return 0.660
	default:
		panic(ErrInvalidMetricValue)
	}
}

func exploitability(v uint8) float64 {
	switch v {
	case e_u:
		return 0.85
	case e_poc:
		return 0.9
	case e_f:
		return 0.95
	case e_h, e_nd:
		return 1.00
	default:
		panic(ErrInvalidMetricValue)
	}
}

func remediationLevel(v uint8) float64 {
	switch v {
	case rl_of:
		return 0.87
	case rl_tf:
		return 0.90
	case rl_w:
		return 0.95
	case rl_u, rl_nd:
		return 1.00
	default:
		panic(ErrInvalidMetricValue)
	}
}

func reportConfidence(v uint8) float64 {
	switch v {
	case rc_uc:
		return 0.90
	case rc_ur:
		return 0.95
	case rc_c, rc_nd:
		return 1.00
	default:
		panic(ErrInvalidMetricValue)
	}
}

func collateralDamagePotential(v uint8) float64 {
	switch v {
	case cdp_n, cdp_nd:
		return 0
	case cdp_l:
		return 0.1
	case cdp_lm:
		return 0.3
	case cdp_mh:
		return 0.4
	case cdp_h:
		return 0.5
	default:
		panic(ErrInvalidMetricValue)
	}
}

func targetDistribution(v uint8) float64 {
	switch v {
	case td_n:
		return 0
	case td_l:
		return 0.25
	case td_m:
		return 0.75
	case td_h, td_nd:
		return 1.00
	default:
		panic(ErrInvalidMetricValue)
	}
}

func ciar(v uint8) float64 {
	switch v {
	case ciar_l:
		return 0.5
	case ciar_m, ciar_nd:
		return 1.0
	case ciar_h:
		return 1.51
	default:
		panic(ErrInvalidMetricValue)
	}
}

// this helper is not specified, so we literally round the value
// to 1 decimal.
func roundTo1Decimal(x float64) float64 {
	return math.Round(x*10) / 10
}
