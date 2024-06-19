package helper

import (
	"evsys-back/internal/lib/api/response"
	"evsys-back/internal/lib/sl"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"log/slog"
	"net/http"
)

type Helper interface {
	GetConfig(name string) (interface{}, error)
}

func Config(logger *slog.Logger, handler Helper) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		name := chi.URLParam(r, "name")

		log := logger.With(
			sl.Module("handlers.helper"),
			slog.String("name", name),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		data, err := handler.GetConfig(name)
		if err != nil {
			log.Error("get config", sl.Err(err))
			render.Status(r, 204)
			render.JSON(w, r, response.Error(2001, fmt.Sprintf("Failed to get config: %v", err)))
			return
		}

		render.JSON(w, r, data)
	}
}
