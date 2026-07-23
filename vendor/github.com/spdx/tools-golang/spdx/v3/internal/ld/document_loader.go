package ld

import (
	"fmt"

	"github.com/piprate/json-gold/ld"
)

type offlineDocumentLoader struct {
	ctx *context
}

func (d offlineDocumentLoader) LoadDocument(u string) (*ld.RemoteDocument, error) {
	sc := d.ctx.contextMap[u]
	if sc != nil {
		return &ld.RemoteDocument{
			DocumentURL: u,
			ContextURL:  u,
			Document:    sc.ldContext,
		}, nil
	}
	return nil, fmt.Errorf("context is not known: %v", u)
}
