package gocvss40

func getDepth(eq int, level int) float64 {
	switch eq {
	case 1:
		switch level {
		case 0:
			return 0 // checked by hand
		case 1:
			return 3 // checked by hand
		case 2:
			return 4 // checked by hand
		}
	case 2:
		switch level {
		case 0:
			return 0 // checked by hand
		case 1:
			return 1 // checked by hand
		}
	case 4:
		switch level {
		case 0:
			return 5 // checked by hand
		case 1:
			return 4 // checked by hand
		case 2:
			return 3 // checked by hand
		}
	case 5:
		// Whatever the level is, it has no deepness (only one metric involved i.e. E)
		return 0 // checked by hand
	}
	// This is a fuzzer hit target, it won't get reached in production :)
	panic("invalid eq value")
}

func getDepthEQ3EQ6(leveleq3, leveleq6 int) float64 {
	switch leveleq3 {
	case 0:
		switch leveleq6 {
		case 0:
			return 6
		case 1:
			return 5
		}
	case 1:
		// Whatever eq6 value, both have a depth of 7
		return 7
	case 2:
		return 9
	}
	// This is a fuzzer hit target, it won't get reached in production :)
	panic("invalid eq value")
}
