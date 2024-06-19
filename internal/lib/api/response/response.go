package response

import "evsys-back/internal/lib/clock"

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
