// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package image

import "time"

var (
	// static is very first commit in github.com/zarf-dev/zarf
	static = time.Date(2021, time.April, 13, 18, 30, 43, 0, time.UTC)
	// format is very first commit in github.com/zarf-dev/zarf as time.RFC3339
	format = time.Date(2021, time.April, 13, 18, 30, 43, 0, time.UTC).Format(time.RFC3339)
)
