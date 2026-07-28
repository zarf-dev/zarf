package rpmdb

const (
	// ref. https://github.com/rpm-software-management/rpm/blob/rpm-4.14.3-release/lib/rpmtag.h#L34
	RPMTAG_HEADERIMAGE      = 61
	RPMTAG_HEADERSIGNATURES = 62
	RPMTAG_HEADERIMMUTABLE  = 63
	HEADER_I18NTABLE        = 100
	RPMTAG_HEADERI18NTABLE  = HEADER_I18NTABLE

	// rpmTag_e
	// ref. https://github.com/rpm-software-management/rpm/blob/rpm-4.14.3-release/lib/rpmtag.h#L34
	RPMTAG_PGP            = 259  /* b */
	RPMTAG_SIGMD5         = 261  /* x */
	RPMTAG_DSAHEADER      = 267  /* x */
	RPMTAG_RSAHEADER      = 268  /* x */
	RPMTAG_SHA1HEADER     = 269  /* s */
	RPMTAG_NAME           = 1000 /* s */
	RPMTAG_VERSION        = 1001 /* s */
	RPMTAG_RELEASE        = 1002 /* s */
	RPMTAG_EPOCH          = 1003 /* i */
	RPMTAG_INSTALLTIME    = 1008 /* i */
	RPMTAG_SIZE           = 1009 /* i */
	RPMTAG_VENDOR         = 1011 /* s */
	RPMTAG_LICENSE        = 1014 /* s */
	RPMTAG_ARCH           = 1022 /* s */
	RPMTAG_FILESIZES      = 1028 /* i[] */
	RPMTAG_FILEMODES      = 1030 /* h[] , specifically []uint16 (ref https://github.com/rpm-software-management/rpm/blob/2153fa4ae51a84547129b8ebb3bb396e1737020e/lib/rpmtypes.h#L53 )*/
	RPMTAG_FILEDIGESTS    = 1035 /* s[] */
	RPMTAG_FILEFLAGS      = 1037 /* i[] */
	RPMTAG_FILEUSERNAME   = 1039 /* s[] */
	RPMTAG_FILEGROUPNAME  = 1040 /* s[] */
	RPMTAG_SOURCERPM      = 1044 /* s */
	RPMTAG_PROVIDENAME    = 1047 /* s[] */
	RPMTAG_REQUIRENAME    = 1049 /* s[] */
	RPMTAG_DIRINDEXES     = 1116 /* i[] */
	RPMTAG_BASENAMES      = 1117 /* s[] */
	RPMTAG_DIRNAMES       = 1118 /* s[] */
	RPMTAG_FILEDIGESTALGO = 5011 /* i  */
	RPMTAG_SUMMARY        = 1004 /* s */

	// rpmTag_enhances
	// https://github.com/rpm-software-management/rpm/blob/rpm-4.16.0-release/lib/rpmtag.h#L375
	RPMTAG_MODULARITYLABEL = 5096

	// rpmTagType_e
	// ref. https://github.com/rpm-software-management/rpm/blob/rpm-4.14.3-release/lib/rpmtag.h#L431
	RPM_MIN_TYPE          = 0
	RPM_NULL_TYPE         = 0
	RPM_CHAR_TYPE         = 1
	RPM_INT8_TYPE         = 2
	RPM_INT16_TYPE        = 3
	RPM_INT32_TYPE        = 4
	RPM_INT64_TYPE        = 5
	RPM_STRING_TYPE       = 6
	RPM_BIN_TYPE          = 7
	RPM_STRING_ARRAY_TYPE = 8
	RPM_I18NSTRING_TYPE   = 9
	RPM_MAX_TYPE          = 9
)
