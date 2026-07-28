package rpmdb

// source: https://github.com/rpm-software-management/rpm/blob/0b75075a8d006c8f792d33a57eae7da6b66a4591/rpmio/rpmpgp.h#L241-L275

type DigestAlgorithm int32

const (
	PGPHASHALGO_MD5         = iota + 1 /*!< MD5 */
	PGPHASHALGO_SHA1                   /*!< SHA1 */
	PGPHASHALGO_RIPEMD160              /*!< RIPEMD160 */
	_                                  /* Reserved for double-width SHA (experimental) */
	PGPHASHALGO_MD2                    /*!< MD2 */
	PGPHASHALGO_TIGER192               /*!< TIGER192 */
	PGPHASHALGO_HAVAL_5_160            /*!< HAVAL-5-160 */
	PGPHASHALGO_SHA256                 /*!< SHA256 */
	PGPHASHALGO_SHA384                 /*!< SHA384 */
	PGPHASHALGO_SHA512                 /*!< SHA512 */
	PGPHASHALGO_SHA224                 /*!< SHA224 */
)

func (d DigestAlgorithm) String() string {
	switch d {
	case 1:
		return "md5"
	case 2:
		return "sha1"
	case 3:
		return "ripemd160"
	case 5:
		return "md2"
	case 6:
		return "tiger192"
	case 7:
		return "haval-5-160"
	case 8:
		return "sha256"
	case 9:
		return "sha384"
	case 10:
		return "sha512"
	case 11:
		return "sha224"
	default:
		return "unknown-digest-algorithm"
	}
}
