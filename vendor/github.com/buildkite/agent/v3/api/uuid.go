package api

import "github.com/pborman/uuid"

func NewUUID() string {
	return uuid.New()
}
