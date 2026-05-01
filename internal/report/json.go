package report

import (
	"encoding/json"
	"io"
)

func WriteJSON(w io.Writer, payload any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(payload)
}
