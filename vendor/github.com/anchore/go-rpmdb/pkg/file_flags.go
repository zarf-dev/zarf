package rpmdb

// source: https://github.com/rpm-software-management/rpm/blob/da55723907418bfb3939cd6ddd941b3fdb4f6887/lib/rpmfiles.h#L46-L63
const (
	RPMFILE_CONFIG    int32 = 1 << iota /*!< from %%config */
	RPMFILE_DOC                         /*!< from %%doc */
	RPMFILE_ICON                        /*!< from %%donotuse. */
	RPMFILE_MISSINGOK                   /*!< from %%config(missingok) */
	RPMFILE_NOREPLACE                   /*!< from %%config(noreplace) */
	RPMFILE_SPECFILE                    /*!< @todo (unnecessary) marks 1st file in srpm. */
	RPMFILE_GHOST                       /*!< from %%ghost */
	RPMFILE_LICENSE                     /*!< from %%license */
	RPMFILE_README                      /*!< from %%readme */
	_                                   /* bit 9 unused */
	_                                   /* bit 10 unused */
	RPMFILE_PUBKEY                      /*!< from %%pubkey */
	RPMFILE_ARTIFACT                    /*!< from %%artifact */
)

var flagChar = map[int32]string{
	RPMFILE_CONFIG:    "c",
	RPMFILE_DOC:       "d",
	RPMFILE_SPECFILE:  "s",
	RPMFILE_MISSINGOK: "m",
	RPMFILE_NOREPLACE: "n",
	RPMFILE_GHOST:     "g",
	RPMFILE_LICENSE:   "l",
	RPMFILE_README:    "r",
	RPMFILE_ARTIFACT:  "a",
}

var flagFormatOrder = []int32{
	RPMFILE_DOC,
	RPMFILE_CONFIG,
	RPMFILE_SPECFILE,
	RPMFILE_MISSINGOK,
	RPMFILE_NOREPLACE,
	RPMFILE_GHOST,
	RPMFILE_LICENSE,
	RPMFILE_README,
	RPMFILE_ARTIFACT,
}

type FileFlags int32

// source: https://github.com/rpm-software-management/rpm/blob/551e66fc94668e62910008d047428eb5ec62f896/lib/verify.c#L298-L312
func (flags FileFlags) String() (result string) {
	for _, flag := range flagFormatOrder {
		if int32(flags)&flag == 0 {
			// flag is NOT set
			continue
		}
		if char, exists := flagChar[flag]; exists {
			result += char
		}
	}
	return
}
