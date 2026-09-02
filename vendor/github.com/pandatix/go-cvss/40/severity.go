package gocvss40

import (
	"fmt"
)

var (
	sevIdx = map[uint8][]uint8{
		// Base metrics
		av: {av_n, av_a, av_l, av_p},
		ac: {ac_l, ac_h},
		at: {at_n, at_p},
		pr: {pr_n, pr_l, pr_h},
		ui: {ui_n, ui_p, ui_a},
		vc: {vscia_h, vscia_l, vscia_n},
		vi: {vscia_h, vscia_l, vscia_n},
		va: {vscia_h, vscia_l, vscia_n},
		sc: {vscia_h, vscia_l, vscia_n},
		si: {vscia_s, vscia_h, vscia_l, vscia_n},
		sa: {vscia_s, vscia_h, vscia_l, vscia_n},
		// Threat metrics
		e: {e_a, e_p, e_u},
		// Environmental metrics
		cr: {ciar_h, ciar_m, ciar_l},
		ir: {ciar_h, ciar_m, ciar_l},
		ar: {ciar_h, ciar_m, ciar_l},
	}
)

// Computes the severity distance between a two values of the same metric.
// Used for regression testing during depths computation.
func severityDistance(metric uint8, vecVal, mxVal uint8) float64 {
	values := sevIdx[metric]
	return index(values, vecVal) - index(values, mxVal)
}

func index(slc []uint8, val uint8) float64 {
	i := 0.
	for _, v := range slc {
		if v == val {
			return i
		}
		i++
	}
	panic(fmt.Sprintf("did not find %v in %v", val, slc))
}
