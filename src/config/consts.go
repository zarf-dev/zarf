// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package config

// These are the CPU architectures Zarf supports building/targeting for.
const (
	// OSArchAMD64 is the x86-64, 64-bit AMD, architecture.
	OSArchAMD64 = "amd64"
	// OSArchARM64 is the 64-bit ARM architecture.
	OSArchARM64 = "arm64"
	// OSArchRISCV is the 64-bit RISC-V architecture.
	OSArchRISCV = "riscv64"
)

// These are the operating systems Zarf supports building/targeting for.
const (
	// OSLinux is the Linux operating system.
	OSLinux = "linux"
	// OSWindows is the Windows operating system.
	OSWindows = "windows"
)
