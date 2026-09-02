// This file is Free Software under the Apache-2.0 License
// without warranty, see README.md and LICENSES/Apache-2.0.txt for details.
//
// SPDX-License-Identifier: Apache-2.0
//
// SPDX-FileCopyrightText: 2023 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
// Software-Engineering: 2023 Intevation GmbH <https://intevation.de>

// Package csaf contains the core data models used by the csaf distribution
// tools.
//
// See https://github.com/gocsaf/csaf/tab=readme-ov-file#use-as-go-library
// about hints and limits for its use as a library.
package csaf

//go:generate go run ./generate_cvss_enums.go -o cvss20enums.go -i ./schema/cvss-v2.0.json -p CVSS20
// Generating only enums for CVSS 3.0 and not for 3.1 since the enums of both of them
// are identical.
//go:generate go run ./generate_cvss_enums.go -o cvss3enums.go -i ./schema/cvss-v3.0.json -p CVSS3
