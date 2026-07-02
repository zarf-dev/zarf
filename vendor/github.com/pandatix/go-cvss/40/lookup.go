package gocvss40

import "fmt"

// For the explanation of those lookup values, refer to Section 8.1.
// Last copy from the official calculator on 18th october 2023.
//
// Transfored to a waterfall-switches for performances and to avoid
// byte manipulations. This last enables github.com/pandatix/gojs-cvss
// to work as dealing with (JS) Number is possible.
//
// Source: https://gist.github.com/pandatix/87e9468c97ed04d454b810b7bce95ba4
func lookupMV(eq1, eq2, eq3, eq4, eq5, eq6 int) float64 {
	switch eq1 {
	case 0:
		switch eq2 {
		case 1:
			switch eq3 {
			case 0:
				switch eq4 {
				case 2:
					switch eq5 {
					case 0:
						switch eq6 {
						case 0:
							return 9.2
						case 1:
							return 8.1
						}
					case 1:
						switch eq6 {
						case 1:
							return 7.1
						case 0:
							return 8.2
						}
					case 2:
						switch eq6 {
						case 0:
							return 7.2
						case 1:
							return 5.3
						}
					}
				case 0:
					switch eq5 {
					case 1:
						switch eq6 {
						case 0:
							return 9.5
						case 1:
							return 9.2
						}
					case 2:
						switch eq6 {
						case 0:
							return 9.2
						case 1:
							return 8.5
						}
					case 0:
						switch eq6 {
						case 1:
							return 9.7
						case 0:
							return 9.9
						}
					}
				case 1:
					switch eq5 {
					case 1:
						switch eq6 {
						case 0:
							return 9.0
						case 1:
							return 8.3
						}
					case 0:
						switch eq6 {
						case 1:
							return 9.1
						case 0:
							return 9.5
						}
					case 2:
						switch eq6 {
						case 1:
							return 7.1
						case 0:
							return 8.4
						}
					}
				}
			case 1:
				switch eq4 {
				case 0:
					switch eq5 {
					case 0:
						switch eq6 {
						case 1:
							return 9.3
						case 0:
							return 9.5
						}
					case 1:
						switch eq6 {
						case 0:
							return 9.2
						case 1:
							return 8.5
						}
					case 2:
						switch eq6 {
						case 0:
							return 8.5
						case 1:
							return 7.3
						}
					}
				case 2:
					switch eq5 {
					case 0:
						switch eq6 {
						case 1:
							return 7.0
						case 0:
							return 8.4
						}
					case 1:
						switch eq6 {
						case 1:
							return 5.2
						case 0:
							return 7.1
						}
					case 2:
						switch eq6 {
						case 1:
							return 3.0
						case 0:
							return 5.0
						}
					}
				case 1:
					switch eq5 {
					case 0:
						switch eq6 {
						case 0:
							return 9.2
						case 1:
							return 8.2
						}
					case 1:
						switch eq6 {
						case 0:
							return 8.0
						case 1:
							return 7.2
						}
					case 2:
						switch eq6 {
						case 1:
							return 5.9
						case 0:
							return 7.0
						}
					}
				}
			case 2:
				switch eq4 {
				case 1:
					switch eq5 {
					case 1:
						switch eq6 {
						case 1:
							return 5.2
						}
					case 0:
						switch eq6 {
						case 1:
							return 7.1
						}
					case 2:
						switch eq6 {
						case 1:
							return 2.9
						}
					}
				case 2:
					switch eq5 {
					case 2:
						switch eq6 {
						case 1:
							return 1.7
						}
					case 1:
						switch eq6 {
						case 1:
							return 2.9
						}
					case 0:
						switch eq6 {
						case 1:
							return 6.3
						}
					}
				case 0:
					switch eq5 {
					case 1:
						switch eq6 {
						case 1:
							return 7.5
						}
					case 2:
						switch eq6 {
						case 1:
							return 5.2
						}
					case 0:
						switch eq6 {
						case 1:
							return 8.6
						}
					}
				}
			}
		case 0:
			switch eq3 {
			case 0:
				switch eq4 {
				case 0:
					switch eq5 {
					case 2:
						switch eq6 {
						case 0:
							return 9.5
						case 1:
							return 9.2
						}
					case 0:
						switch eq6 {
						case 1:
							return 9.9
						case 0:
							return 10.0
						}
					case 1:
						switch eq6 {
						case 0:
							return 9.8
						case 1:
							return 9.5
						}
					}
				case 2:
					switch eq5 {
					case 2:
						switch eq6 {
						case 1:
							return 6.8
						case 0:
							return 8.1
						}
					case 1:
						switch eq6 {
						case 0:
							return 8.9
						case 1:
							return 8.0
						}
					case 0:
						switch eq6 {
						case 1:
							return 9.0
						case 0:
							return 9.3
						}
					}
				case 1:
					switch eq5 {
					case 0:
						switch eq6 {
						case 0:
							return 10.0
						case 1:
							return 9.6
						}
					case 2:
						switch eq6 {
						case 0:
							return 9.1
						case 1:
							return 8.1
						}
					case 1:
						switch eq6 {
						case 1:
							return 8.7
						case 0:
							return 9.3
						}
					}
				}
			case 1:
				switch eq4 {
				case 1:
					switch eq5 {
					case 1:
						switch eq6 {
						case 0:
							return 8.9
						case 1:
							return 8.1
						}
					case 2:
						switch eq6 {
						case 1:
							return 6.5
						case 0:
							return 8.1
						}
					case 0:
						switch eq6 {
						case 1:
							return 9.2
						case 0:
							return 9.3
						}
					}
				case 2:
					switch eq5 {
					case 2:
						switch eq6 {
						case 0:
							return 6.9
						case 1:
							return 4.8
						}
					case 0:
						switch eq6 {
						case 1:
							return 8.0
						case 0:
							return 8.8
						}
					case 1:
						switch eq6 {
						case 1:
							return 7.0
						case 0:
							return 7.8
						}
					}
				case 0:
					switch eq5 {
					case 0:
						switch eq6 {
						case 0:
							return 9.8
						case 1:
							return 9.5
						}
					case 1:
						switch eq6 {
						case 0:
							return 9.5
						case 1:
							return 9.2
						}
					case 2:
						switch eq6 {
						case 1:
							return 8.4
						case 0:
							return 9.0
						}
					}
				}
			case 2:
				switch eq4 {
				case 0:
					switch eq5 {
					case 2:
						switch eq6 {
						case 1:
							return 7.2
						}
					case 0:
						switch eq6 {
						case 1:
							return 9.2
						}
					case 1:
						switch eq6 {
						case 1:
							return 8.2
						}
					}
				case 2:
					switch eq5 {
					case 0:
						switch eq6 {
						case 1:
							return 6.9
						}
					case 1:
						switch eq6 {
						case 1:
							return 5.5
						}
					case 2:
						switch eq6 {
						case 1:
							return 2.7
						}
					}
				case 1:
					switch eq5 {
					case 0:
						switch eq6 {
						case 1:
							return 7.9
						}
					case 1:
						switch eq6 {
						case 1:
							return 6.9
						}
					case 2:
						switch eq6 {
						case 1:
							return 5.0
						}
					}
				}
			}
		}
	case 1:
		switch eq2 {
		case 0:
			switch eq3 {
			case 1:
				switch eq4 {
				case 0:
					switch eq5 {
					case 0:
						switch eq6 {
						case 1:
							return 8.9
						case 0:
							return 9.4
						}
					case 2:
						switch eq6 {
						case 1:
							return 6.7
						case 0:
							return 7.6
						}
					case 1:
						switch eq6 {
						case 1:
							return 7.7
						case 0:
							return 8.8
						}
					}
				case 2:
					switch eq5 {
					case 2:
						switch eq6 {
						case 0:
							return 5.2
						case 1:
							return 2.5
						}
					case 1:
						switch eq6 {
						case 0:
							return 5.7
						case 1:
							return 5.2
						}
					case 0:
						switch eq6 {
						case 0:
							return 7.2
						case 1:
							return 5.7
						}
					}
				case 1:
					switch eq5 {
					case 2:
						switch eq6 {
						case 1:
							return 5.0
						case 0:
							return 5.9
						}
					case 1:
						switch eq6 {
						case 1:
							return 5.8
						case 0:
							return 7.4
						}
					case 0:
						switch eq6 {
						case 1:
							return 7.6
						case 0:
							return 8.6
						}
					}
				}
			case 0:
				switch eq4 {
				case 1:
					switch eq5 {
					case 2:
						switch eq6 {
						case 0:
							return 7.7
						case 1:
							return 6.4
						}
					case 1:
						switch eq6 {
						case 1:
							return 7.4
						case 0:
							return 8.6
						}
					case 0:
						switch eq6 {
						case 1:
							return 8.9
						case 0:
							return 9.4
						}
					}
				case 2:
					switch eq5 {
					case 0:
						switch eq6 {
						case 0:
							return 8.7
						case 1:
							return 7.5
						}
					case 2:
						switch eq6 {
						case 1:
							return 4.9
						case 0:
							return 6.3
						}
					case 1:
						switch eq6 {
						case 0:
							return 7.4
						case 1:
							return 6.3
						}
					}
				case 0:
					switch eq5 {
					case 0:
						switch eq6 {
						case 0:
							return 9.8
						case 1:
							return 9.5
						}
					case 2:
						switch eq6 {
						case 1:
							return 8.1
						case 0:
							return 9.1
						}
					case 1:
						switch eq6 {
						case 1:
							return 8.7
						case 0:
							return 9.4
						}
					}
				}
			case 2:
				switch eq4 {
				case 0:
					switch eq5 {
					case 2:
						switch eq6 {
						case 1:
							return 5.4
						}
					case 1:
						switch eq6 {
						case 1:
							return 7.0
						}
					case 0:
						switch eq6 {
						case 1:
							return 8.3
						}
					}
				case 2:
					switch eq5 {
					case 0:
						switch eq6 {
						case 1:
							return 5.3
						}
					case 2:
						switch eq6 {
						case 1:
							return 1.3
						}
					case 1:
						switch eq6 {
						case 1:
							return 2.1
						}
					}
				case 1:
					switch eq5 {
					case 1:
						switch eq6 {
						case 1:
							return 5.8
						}
					case 2:
						switch eq6 {
						case 1:
							return 2.6
						}
					case 0:
						switch eq6 {
						case 1:
							return 6.5
						}
					}
				}
			}
		case 1:
			switch eq3 {
			case 0:
				switch eq4 {
				case 1:
					switch eq5 {
					case 1:
						switch eq6 {
						case 1:
							return 6.2
						case 0:
							return 7.5
						}
					case 2:
						switch eq6 {
						case 0:
							return 6.1
						case 1:
							return 5.3
						}
					case 0:
						switch eq6 {
						case 0:
							return 9.0
						case 1:
							return 7.7
						}
					}
				case 0:
					switch eq5 {
					case 0:
						switch eq6 {
						case 1:
							return 9.0
						case 0:
							return 9.5
						}
					case 1:
						switch eq6 {
						case 0:
							return 8.8
						case 1:
							return 7.6
						}
					case 2:
						switch eq6 {
						case 0:
							return 7.6
						case 1:
							return 7.0
						}
					}
				case 2:
					switch eq5 {
					case 2:
						switch eq6 {
						case 0:
							return 5.2
						case 1:
							return 3.0
						}
					case 0:
						switch eq6 {
						case 1:
							return 6.6
						case 0:
							return 7.7
						}
					case 1:
						switch eq6 {
						case 1:
							return 5.9
						case 0:
							return 6.8
						}
					}
				}
			case 2:
				switch eq4 {
				case 0:
					switch eq5 {
					case 2:
						switch eq6 {
						case 1:
							return 3.0
						}
					case 0:
						switch eq6 {
						case 1:
							return 7.1
						}
					case 1:
						switch eq6 {
						case 1:
							return 5.9
						}
					}
				case 2:
					switch eq5 {
					case 1:
						switch eq6 {
						case 1:
							return 1.3
						}
					case 0:
						switch eq6 {
						case 1:
							return 2.3
						}
					case 2:
						switch eq6 {
						case 1:
							return 0.6
						}
					}
				case 1:
					switch eq5 {
					case 1:
						switch eq6 {
						case 1:
							return 2.6
						}
					case 0:
						switch eq6 {
						case 1:
							return 5.8
						}
					case 2:
						switch eq6 {
						case 1:
							return 1.5
						}
					}
				}
			case 1:
				switch eq4 {
				case 0:
					switch eq5 {
					case 0:
						switch eq6 {
						case 0:
							return 8.9
						case 1:
							return 7.8
						}
					case 2:
						switch eq6 {
						case 0:
							return 6.2
						case 1:
							return 5.8
						}
					case 1:
						switch eq6 {
						case 1:
							return 6.7
						case 0:
							return 7.6
						}
					}
				case 2:
					switch eq5 {
					case 0:
						switch eq6 {
						case 1:
							return 5.2
						case 0:
							return 6.1
						}
					case 2:
						switch eq6 {
						case 0:
							return 2.4
						case 1:
							return 1.6
						}
					case 1:
						switch eq6 {
						case 0:
							return 5.7
						case 1:
							return 2.9
						}
					}
				case 1:
					switch eq5 {
					case 1:
						switch eq6 {
						case 0:
							return 5.7
						case 1:
							return 5.7
						}
					case 2:
						switch eq6 {
						case 1:
							return 2.3
						case 0:
							return 4.7
						}
					case 0:
						switch eq6 {
						case 1:
							return 5.9
						case 0:
							return 7.4
						}
					}
				}
			}
		}
	case 2:
		switch eq2 {
		case 1:
			switch eq3 {
			case 2:
				switch eq4 {
				case 2:
					switch eq5 {
					case 0:
						switch eq6 {
						case 1:
							return 1.0
						}
					case 1:
						switch eq6 {
						case 1:
							return 0.3
						}
					case 2:
						switch eq6 {
						case 1:
							return 0.1
						}
					}
				case 0:
					switch eq5 {
					case 1:
						switch eq6 {
						case 1:
							return 2.4
						}
					case 2:
						switch eq6 {
						case 1:
							return 1.4
						}
					case 0:
						switch eq6 {
						case 1:
							return 5.3
						}
					}
				case 1:
					switch eq5 {
					case 1:
						switch eq6 {
						case 1:
							return 1.2
						}
					case 0:
						switch eq6 {
						case 1:
							return 2.4
						}
					case 2:
						switch eq6 {
						case 1:
							return 0.5
						}
					}
				}
			case 0:
				switch eq4 {
				case 2:
					switch eq5 {
					case 0:
						switch eq6 {
						case 0:
							return 5.4
						case 1:
							return 4.3
						}
					case 1:
						switch eq6 {
						case 1:
							return 2.2
						case 0:
							return 4.5
						}
					case 2:
						switch eq6 {
						case 1:
							return 1.1
						case 0:
							return 2.0
						}
					}
				case 0:
					switch eq5 {
					case 2:
						switch eq6 {
						case 0:
							return 6.0
						case 1:
							return 5.0
						}
					case 0:
						switch eq6 {
						case 1:
							return 7.5
						case 0:
							return 8.8
						}
					case 1:
						switch eq6 {
						case 0:
							return 7.3
						case 1:
							return 5.3
						}
					}
				case 1:
					switch eq5 {
					case 0:
						switch eq6 {
						case 1:
							return 5.5
						case 0:
							return 7.3
						}
					case 1:
						switch eq6 {
						case 1:
							return 4.0
						case 0:
							return 5.9
						}
					case 2:
						switch eq6 {
						case 0:
							return 4.1
						case 1:
							return 2.0
						}
					}
				}
			case 1:
				switch eq4 {
				case 0:
					switch eq5 {
					case 2:
						switch eq6 {
						case 0:
							return 4.0
						case 1:
							return 2.1
						}
					case 1:
						switch eq6 {
						case 0:
							return 5.8
						case 1:
							return 4.5
						}
					case 0:
						switch eq6 {
						case 1:
							return 5.5
						case 0:
							return 7.5
						}
					}
				case 2:
					switch eq5 {
					case 0:
						switch eq6 {
						case 0:
							return 4.6
						case 1:
							return 1.8
						}
					case 1:
						switch eq6 {
						case 1:
							return 0.7
						case 0:
							return 1.7
						}
					case 2:
						switch eq6 {
						case 0:
							return 0.8
						case 1:
							return 0.2
						}
					}
				case 1:
					switch eq5 {
					case 2:
						switch eq6 {
						case 1:
							return 0.9
						case 0:
							return 2.0
						}
					case 1:
						switch eq6 {
						case 0:
							return 4.8
						case 1:
							return 1.8
						}
					case 0:
						switch eq6 {
						case 0:
							return 6.1
						case 1:
							return 5.1
						}
					}
				}
			}
		case 0:
			switch eq3 {
			case 0:
				switch eq4 {
				case 1:
					switch eq5 {
					case 1:
						switch eq6 {
						case 1:
							return 6.1
						case 0:
							return 7.4
						}
					case 2:
						switch eq6 {
						case 0:
							return 5.6
						case 1:
							return 3.4
						}
					case 0:
						switch eq6 {
						case 1:
							return 7.4
						case 0:
							return 8.6
						}
					}
				case 0:
					switch eq5 {
					case 0:
						switch eq6 {
						case 1:
							return 8.7
						case 0:
							return 9.3
						}
					case 2:
						switch eq6 {
						case 0:
							return 7.5
						case 1:
							return 5.8
						}
					case 1:
						switch eq6 {
						case 1:
							return 7.2
						case 0:
							return 8.6
						}
					}
				case 2:
					switch eq5 {
					case 1:
						switch eq6 {
						case 1:
							return 4.0
						case 0:
							return 5.2
						}
					case 0:
						switch eq6 {
						case 0:
							return 7.0
						case 1:
							return 5.4
						}
					case 2:
						switch eq6 {
						case 1:
							return 2.2
						case 0:
							return 4.0
						}
					}
				}
			case 1:
				switch eq4 {
				case 1:
					switch eq5 {
					case 2:
						switch eq6 {
						case 0:
							return 4.6
						case 1:
							return 1.9
						}
					case 0:
						switch eq6 {
						case 0:
							return 7.2
						case 1:
							return 5.7
						}
					case 1:
						switch eq6 {
						case 1:
							return 4.1
						case 0:
							return 5.5
						}
					}
				case 2:
					switch eq5 {
					case 1:
						switch eq6 {
						case 1:
							return 1.9
						case 0:
							return 3.4
						}
					case 2:
						switch eq6 {
						case 0:
							return 1.9
						case 1:
							return 0.8
						}
					case 0:
						switch eq6 {
						case 0:
							return 5.3
						case 1:
							return 3.6
						}
					}
				case 0:
					switch eq5 {
					case 2:
						switch eq6 {
						case 1:
							return 5.1
						case 0:
							return 6.2
						}
					case 1:
						switch eq6 {
						case 0:
							return 7.4
						case 1:
							return 5.5
						}
					case 0:
						switch eq6 {
						case 0:
							return 8.5
						case 1:
							return 7.5
						}
					}
				}
			case 2:
				switch eq4 {
				case 1:
					switch eq5 {
					case 0:
						switch eq6 {
						case 1:
							return 4.7
						}
					case 1:
						switch eq6 {
						case 1:
							return 2.1
						}
					case 2:
						switch eq6 {
						case 1:
							return 1.1
						}
					}
				case 0:
					switch eq5 {
					case 0:
						switch eq6 {
						case 1:
							return 6.4
						}
					case 1:
						switch eq6 {
						case 1:
							return 5.1
						}
					case 2:
						switch eq6 {
						case 1:
							return 2.0
						}
					}
				case 2:
					switch eq5 {
					case 0:
						switch eq6 {
						case 1:
							return 2.4
						}
					case 2:
						switch eq6 {
						case 1:
							return 0.4
						}
					case 1:
						switch eq6 {
						case 1:
							return 0.9
						}
					}
				}
			}
		}
	}
	panic(fmt.Sprintf("invalid EQs combination: %d %d %d %d %d %d", eq1, eq2, eq3, eq4, eq5, eq6))
}
