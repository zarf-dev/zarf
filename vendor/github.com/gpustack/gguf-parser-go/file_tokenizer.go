package gguf_parser

// GGUFTokenizer represents the tokenizer metadata of a GGUF file.
type GGUFTokenizer struct {
	/* Basic */

	// Model is the model of the tokenizer.
	Model string `json:"model"`
	// TokensLength is the size of tokens.
	TokensLength uint64 `json:"tokensLength"`
	// MergeLength is the size of merges.
	MergesLength uint64 `json:"mergesLength"`
	// AddedTokensLength is the size of added tokens after training.
	AddedTokensLength uint64 `json:"addedTokenLength"`
	// BOSTokenID is the ID of the beginning of sentence token.
	//
	// Use -1 if the token is not found.
	BOSTokenID int64 `json:"bosTokenID"`
	// EOSTokenID is the ID of the end of sentence token.
	//
	// Use -1 if the token is not found.
	EOSTokenID int64 `json:"eosTokenID"`
	// EOTTokenID is the ID of the end of text token.
	//
	// Use -1 if the token is not found.
	EOTTokenID int64 `json:"eotTokenID"`
	// EOMTokenID is the ID of the end of message token.
	//
	// Use -1 if the token is not found.
	EOMTokenID int64 `json:"eomTokenID"`
	// UnknownTokenID is the ID of the unknown token.
	//
	// Use -1 if the token is not found.
	UnknownTokenID int64 `json:"unknownTokenID"`
	// SeparatorTokenID is the ID of the separator token.
	//
	// Use -1 if the token is not found.
	SeparatorTokenID int64 `json:"separatorTokenID"`
	// PaddingTokenID is the ID of the padding token.
	//
	// Use -1 if the token is not found.
	PaddingTokenID int64 `json:"paddingTokenID"`

	/* Appendix */

	// TokenSize is the size of tokens in bytes.
	TokensSize int64 `json:"tokensSize"`
	// MergesSize is the size of merges in bytes.
	MergesSize int64 `json:"mergesSize"`
}

// Tokenizer returns the tokenizer metadata of a GGUF file.
func (gf *GGUFFile) Tokenizer() (gt GGUFTokenizer) {
	const (
		modelKey            = "tokenizer.ggml.model"
		tokensKey           = "tokenizer.ggml.tokens"
		mergesKey           = "tokenizer.ggml.merges"
		addedTokensKey      = "tokenizer.ggml.added_tokens"
		bosTokenIDKey       = "tokenizer.ggml.bos_token_id"
		eosTokenIDKey       = "tokenizer.ggml.eos_token_id"
		eotTokenIDKey       = "tokenizer.ggml.eot_token_id"
		eomTokenIDKey       = "tokenizer.ggml.eom_token_id"
		unknownTokenIDKey   = "tokenizer.ggml.unknown_token_id"
		separatorTokenIDKey = "tokenizer.ggml.separator_token_id"
		paddingTokenIDKey   = "tokenizer.ggml.padding_token_id"
	)

	m, _ := gf.Header.MetadataKV.Index([]string{
		modelKey,
		tokensKey,
		mergesKey,
		addedTokensKey,
		bosTokenIDKey,
		eosTokenIDKey,
		eotTokenIDKey,
		eomTokenIDKey,
		unknownTokenIDKey,
		separatorTokenIDKey,
		paddingTokenIDKey,
	})

	gt.BOSTokenID = -1
	gt.EOSTokenID = -1
	gt.EOTTokenID = -1
	gt.EOMTokenID = -1
	gt.UnknownTokenID = -1
	gt.SeparatorTokenID = -1
	gt.PaddingTokenID = -1

	if v, ok := m[modelKey]; ok {
		gt.Model = v.ValueString()
	}
	if v, ok := m[tokensKey]; ok {
		arr := v.ValueArray()
		gt.TokensLength = arr.Len
		gt.TokensSize = arr.Size
	}
	if v, ok := m[mergesKey]; ok {
		arr := v.ValueArray()
		gt.MergesLength = arr.Len
		gt.MergesSize = arr.Size
	}
	if v, ok := m[addedTokensKey]; ok {
		gt.AddedTokensLength = v.ValueArray().Len
	}
	if v, ok := m[bosTokenIDKey]; ok {
		gt.BOSTokenID = ValueNumeric[int64](v)
	}
	if v, ok := m[eosTokenIDKey]; ok {
		gt.EOSTokenID = ValueNumeric[int64](v)
	}
	if v, ok := m[eotTokenIDKey]; ok {
		gt.EOTTokenID = ValueNumeric[int64](v)
	}
	if v, ok := m[eomTokenIDKey]; ok {
		gt.EOMTokenID = ValueNumeric[int64](v)
	}
	if v, ok := m[unknownTokenIDKey]; ok {
		gt.UnknownTokenID = ValueNumeric[int64](v)
	}
	if v, ok := m[separatorTokenIDKey]; ok {
		gt.SeparatorTokenID = ValueNumeric[int64](v)
	}
	if v, ok := m[paddingTokenIDKey]; ok {
		gt.PaddingTokenID = ValueNumeric[int64](v)
	}

	return gt
}
