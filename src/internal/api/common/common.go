package common

import (
	"encoding/json"
	"net/http"

	"github.com/defenseunicorns/zarf/src/internal/message"
)

func WriteEmpty(w http.ResponseWriter) {
	message.Debug("api.WriteEmpty()")
	w.WriteHeader(http.StatusNoContent)
}

func WriteJSONResponse(w http.ResponseWriter, statusCode int, data any) {
	message.Debug("api.WriteJSONResponse()")
	encoded, err := json.Marshal(data)
	if err != nil {
		message.Error(err, "Error marshalling JSON")
		panic(err)
	}

	w.WriteHeader(statusCode)
	w.Write(encoded)
}
