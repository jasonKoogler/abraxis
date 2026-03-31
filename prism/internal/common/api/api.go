package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/jasonKoogler/prism/internal/common/log"
)

// func UserFromContext(ctx context.Context) (*authDomain.User, error) {
// 	user, err := auth.UserFromContext(ctx)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return user, nil
// }

func Respond(w http.ResponseWriter, code int, v interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	return json.NewEncoder(w).Encode(v)
}

func LimitAndOffsetFromRequest(r *http.Request) (int, int, error) {
	limitString := r.URL.Query().Get("limit")
	if limitString == "" {
		return 0, 0, ErrInvalidLimit
	}

	offsetString := r.URL.Query().Get("offset")
	if offsetString == "" {
		return 0, 0, ErrInvalidOffset
	}

	limit, err := strconv.Atoi(limitString)
	if err != nil {
		return 0, 0, ErrInvalidLimit
	}

	offset, err := strconv.Atoi(offsetString)
	if err != nil {
		return 0, 0, ErrInvalidOffset
	}

	return limit, offset, nil
}

// ParsePageSize returns the page size, defaulting to 10 if the page size is not provided or is less than 1
func ParsePageSize(pageSize *int) int {
	if pageSize == nil || *pageSize < 1 {
		return 10
	}

	return *pageSize
}

// ParsePage returns the page, defaulting to 1 if the page is not provided or is less than 1
func ParsePage(page *int) int {
	if page == nil || *page < 1 {
		return 1
	}

	return *page
}

// ParsePagination parses the page and page size, defaulting to 1 and 10 if they are not provided
func ParsePagination(page, pageSize *int) (int, int) {
	return ParsePage(page), ParsePageSize(pageSize)
}

func ParseStartTimeEndTime(startTime time.Time, endTime time.Time) (time.Time, time.Time) {
	if startTime.IsZero() {
		startTime = time.Now()
	}

	if endTime.IsZero() {
		endTime = startTime.Add(time.Hour * 24)
	}

	return startTime, endTime
}

// BindRequest binds the request body to the provided struct
func BindRequest(r *http.Request, v interface{}) error {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return err
	}
	return nil
}

func Redirect(w http.ResponseWriter, r *http.Request, url string) {
	w.Header().Set("Location", url)
	w.WriteHeader(http.StatusFound)
}

type APIFunc func(w http.ResponseWriter, r *http.Request) error

func Make(h APIFunc, logger *log.Logger) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		if err := h(w, r); err != nil {
			RespondWithError(err, w, r)
			logger.Error("HTTP API error", log.Error(err), log.String("path", r.URL.Path))
		}
	}
}

var (
	ErrInvalidLimit    = errors.New("invalid limit")
	ErrInvalidOffset   = errors.New("invalid offset")
	ErrInvalidPageSize = errors.New("invalid page size")
)
