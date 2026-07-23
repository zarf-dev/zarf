package gocvss40

// Heading 0 are removed, else it is interpreted as octal
// values by Go.

var highestSeverityVectors = map[int]map[int][]int{
	// Table 24 - EQ1
	1: {
		0: []int{20}, // AV:N/PR:N/UI:N
		1: []int{
			120, // AV:A/PR:N/UI:N
			10,  // AV:N/PR:L/UI:N
			21,  // AV:N/PR:N/UI:P
		},
		2: []int{
			320, // AV:P/PR:N/UI:N
			111, // AV:A/PR:L/UI:P
		},
	},
	// Table 25 - EQ2
	2: {
		0: []int{10}, // AC:L/AT:N
		1: []int{
			11, // AC:L/AT:P
			0,  // AC:H/AT:N
		},
	},
	// Table 27 - EQ4
	// Use MSI and MSA
	4: {
		0: []int{33},  // SC:H/SI:S/SA:S
		1: []int{0},   // SC:H/SI:H/SA:H
		2: []int{111}, // SC:L/SI:L/SA:L
	},
	// Table 28 - EQ5
	5: {
		0: []int{1}, // E:A
		1: []int{2}, // E:P
		2: []int{3}, // E:U
	},
}

// Table 30
var highestSeverityVectorsEQ3EQ6 = map[int]map[int][]int{
	0: {
		0: []int{111}, //  VC:H/VI:H/VA:H/CR:H/IR:H/AR:H
		1: []int{
			1221, // VC:H/VI:H/VA:L/CR:M/IR:M/AR:H
			222,  // VC:H/VI:H/VA:H/CR:M/IR:M/AR:M
		},
	},
	1: {
		0: []int{
			100111, // VC:L/VI:H/VA:H/CR:H/IR:H/AR:H
			10111,  // VC:H/VI:L/VA:H/CR:H/IR:H/AR:H
		},
		1: []int{
			10212,  // VC:H/VI:L/VA:H/CR:M/IR:H/AR:M
			11211,  // VC:H/VI:L/VA:L/CR:M/IR:H/AR:H
			100122, // VC:L/VI:H/VA:H/CR:H/IR:M/AR:M
			101121, // VC:L/VI:H/VA:L/CR:H/IR:M/AR:H
			110112, // VC:L/VI:L/VA:H/CR:H/IR:H/AR:M
		},
	},
	2: {
		1: []int{111111}, // VC:L/VI:L/VA:L/CR:H/IR:H/AR:H
	},
}
