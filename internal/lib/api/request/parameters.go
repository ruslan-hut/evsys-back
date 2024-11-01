package request

import (
	"evsys-back/internal/lib/validate"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func GetDate(r *http.Request, key string) (time.Time, error) {
	query := r.URL.Query()
	par := query.Get(key)
	if par == "" {
		return time.Now(), fmt.Errorf("parameter '%s' is missing", key)
	}
	initial := par
	if len(par) == 10 {
		par += "T00:00:00Z"
	}
	if !strings.HasSuffix(par, "Z") {
		par += "Z"
	}
	date, err := validate.DateString(par)
	if err != nil {
		return time.Now(), fmt.Errorf("%s=%s: cannot parse as date", key, initial)
	}
	return date, nil
}

func GetString(r *http.Request, key string) (string, error) {
	query := r.URL.Query()
	par := query.Get(key)
	if par == "" {
		return "", fmt.Errorf("parameter '%s' is missing", key)
	}
	return par, nil
}

func GetNumber(r *http.Request, key string) (string, error) {
	query := r.URL.Query()
	par := query.Get(key)
	if par == "" {
		return "", nil
	}
	_, err := strconv.Atoi(par)
	if err != nil {
		return "", fmt.Errorf("%s=%s: cannot parse as number", key, par)
	}
	return par, nil
}
