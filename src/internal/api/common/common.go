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

func WriteJSONResponse(w http.ResponseWriter, data any) {
	message.Debug("api.WriteJSONResponse()")
	message.JsonValue(data)
	encoded, err := json.Marshal(data)
	if err != nil {
		message.Error(err, "Error marshalling JSON")
		panic(err)
	}
	w.Write(encoded)
}
