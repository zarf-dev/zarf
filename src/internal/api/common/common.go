package common

import (
	"encoding/json"
	"net/http"

	"github.com/defenseunicorns/zarf/src/internal/message"
)

// WriteEmpty returns a 204 response with no body.
func WriteEmpty(w http.ResponseWriter) {
	message.Debug("api.WriteEmpty()")
	w.WriteHeader(http.StatusNoContent)
}

// WriteJSONResponse returns any data provided as a JSON body to the caller.
func WriteJSONResponse(w http.ResponseWriter, data any, statusCode int) {
	message.Debug("api.WriteJSONResponse()")

	var encoded []byte
	var err error
	if data == nil {
		encoded, err = json.Marshal(data)
		if err != nil {
			message.Error(err, "Error marshalling JSON")
			panic(err)
		}
	}

	w.WriteHeader(statusCode)
	w.Write(encoded)
}
