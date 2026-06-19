// Package web holds the small request-handling helpers shared by every REST
// handler: building the request-scoped logger and writing success/error
// responses. They collapse the boilerplate that otherwise repeats in every
// endpoint without hiding each handler's status codes or control flow.
package web

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"evsys-back/entity"
	"evsys-back/internal/lib/api/response"
	"evsys-back/internal/lib/sl"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
)

// Log returns a request-scoped logger preloaded with the module name, the
// caller-supplied fields, and the request id.
func Log(ctx context.Context, logger *slog.Logger, module string, fields ...any) *slog.Logger {
	attrs := make([]any, 0, len(fields)+2)
	attrs = append(attrs, sl.Module(module))
	attrs = append(attrs, fields...)
	attrs = append(attrs, slog.String("request_id", middleware.GetReqID(ctx)))
	return logger.With(attrs...)
}

// Fail logs err and writes a JSON error response with application code 2001.
// When status <= 0 it is derived from the error: 404 for entity.ErrNotFound,
// otherwise 400.
func Fail(w http.ResponseWriter, r *http.Request, log *slog.Logger, status int, msg string, err error) {
	FailCode(w, r, log, status, 2001, msg, err)
}

// FailCode is Fail with an explicit application error code.
func FailCode(w http.ResponseWriter, r *http.Request, log *slog.Logger, status, code int, msg string, err error) {
	if status <= 0 {
		status = http.StatusBadRequest
		if errors.Is(err, entity.ErrNotFound) {
			status = http.StatusNotFound
		}
	}
	if err != nil {
		log = log.With(sl.Err(err))
	}
	log.Error(msg)
	response.RenderErr(w, r, status, code, msg, err)
}

// OK logs msg at info level and writes data as a JSON response.
func OK(w http.ResponseWriter, r *http.Request, log *slog.Logger, msg string, data any) {
	log.Info(msg)
	render.JSON(w, r, data)
}

// Created behaves like OK but sets HTTP status 201.
func Created(w http.ResponseWriter, r *http.Request, log *slog.Logger, msg string, data any) {
	render.Status(r, http.StatusCreated)
	OK(w, r, log, msg, data)
}
