package registry

import (
	"encoding/json"
	"net/http"
)

type regError struct {
	Status  int
	Code    string
	Message string
}

func (r *regError) Write(resp http.ResponseWriter) error {
	resp.WriteHeader(r.Status)

	type err struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	type wrap struct {
		Errors []err `json:"errors"`
	}
	return json.NewEncoder(resp).Encode(wrap{
		Errors: []err{
			{
				Code:    r.Code,
				Message: r.Message,
			},
		},
	})
}
