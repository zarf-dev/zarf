/*
 * Copyright (c) SAS Institute, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package rpmutils

// PGP Hash Algorithms
const (
	PGPHASHALGO_MD5         = 1  // MD5
	PGPHASHALGO_SHA1        = 2  // SHA1
	PGPHASHALGO_RIPEMD160   = 3  // RIPEMD160
	PGPHASHALGO_MD2         = 5  // MD2
	PGPHASHALGO_TIGER192    = 6  // TIGER192
	PGPHASHALGO_HAVAL_5_160 = 7  // HAVAL-5-160
	PGPHASHALGO_SHA256      = 8  // SHA256
	PGPHASHALGO_SHA384      = 9  // SHA384
	PGPHASHALGO_SHA512      = 10 // SHA512
	PGPHASHALGO_SHA224      = 11 // SHA224
)

// GetFileAlgoName returns the name of a digest algorithm
func GetFileAlgoName(algo int) string {
	switch algo {
	case PGPHASHALGO_MD5:
		return "md5"
	case PGPHASHALGO_SHA1:
		return "sha1"
	case PGPHASHALGO_RIPEMD160:
		return "rmd160"
	case PGPHASHALGO_MD2:
		return "md2"
	case PGPHASHALGO_TIGER192:
		return "tgr192"
	case PGPHASHALGO_HAVAL_5_160:
		return "haval5160"
	case PGPHASHALGO_SHA256:
		return "sha256"
	case PGPHASHALGO_SHA384:
		return "sha384"
	case PGPHASHALGO_SHA512:
		return "sha512"
	case PGPHASHALGO_SHA224:
		return "sha224"
	default:
		return "md5"
	}
}
