package response

import (
	"evsys-back/internal/lib/clock"
	"fmt"
	"net/http"

	"github.com/go-chi/render"
)

type Response struct {
	Data          interface{} `json:"data,omitempty"`
	StatusCode    int         `json:"status_code" validate:"required,min=1000,max=3003"`
	StatusMessage string      `json:"status_message"`
	Timestamp     string      `json:"timestamp"`
}

func Ok(data interface{}) Response {
	return Response{
		Data:          data,
		StatusCode:    1000,
		StatusMessage: "Success",
		Timestamp:     clock.Now(),
	}
}

func Error(code int, message string) Response {
	return Response{
		StatusCode:    code,
		StatusMessage: message,
		Timestamp:     clock.Now(),
	}
}

// RenderErr writes a JSON error response with formatted error detail.
func RenderErr(w http.ResponseWriter, r *http.Request, status, code int, msg string, err error) {
	render.Status(r, status)
	render.JSON(w, r, Error(code, fmt.Sprintf("%s: %v", msg, err)))
}

// Render writes a JSON error response with a plain message.
func Render(w http.ResponseWriter, r *http.Request, status, code int, msg string) {
	render.Status(r, status)
	render.JSON(w, r, Error(code, msg))
}

// Forbidden writes a 403 Forbidden response.
func Forbidden(w http.ResponseWriter, r *http.Request) {
	render.Status(r, http.StatusForbidden)
	render.JSON(w, r, Error(2001, "Insufficient permissions"))
}
