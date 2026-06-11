package http

import (
	"net/http"

	"github.com/swaggo/swag"
)

func SwaggerDocHandler(w http.ResponseWriter, r *http.Request) {
	doc, err := swag.ReadDoc()
	if err != nil {
		http.Error(w, "spec unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Write([]byte(doc))
}
