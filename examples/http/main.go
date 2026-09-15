package main

import (
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/im-wmkong/errkind"
	httperr "github.com/im-wmkong/errkind/http"
	slogerr "github.com/im-wmkong/errkind/slog"
)

var (
	UserNotFound    = errkind.Define(10001, "user_not_found")
	InvalidArgument = errkind.Define(10002, "invalid_argument")
	Internal        = errkind.Define(10500, "internal")
)

var errNoRows = errors.New("sql: no rows in result set")

func getUser(id int64) error {
	switch {
	case id <= 0:
		return InvalidArgument.New("id must be positive", errkind.With("id", id))
	case id == 999:
		return UserNotFound.Wrap(errNoRows, "lookup user", errkind.With("uid", id))
	case id == 500:
		return Internal.Wrap(errors.New("database connection failed"), "lookup user")
	default:
		return nil
	}
}

func handleUser(logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, parseErr := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		var err error
		if parseErr != nil {
			err = InvalidArgument.Wrap(parseErr, "parse user id")
		} else {
			err = getUser(id)
		}
		if err != nil {
			code := http.StatusInternalServerError
			var opts []httperr.Option
			switch errkind.KindOf(err) {
			case InvalidArgument:
				code = http.StatusBadRequest
				opts = []httperr.Option{httperr.Status(code), httperr.Message("id 必须是正整数"), httperr.Identity()}
			case UserNotFound:
				code = http.StatusNotFound
				opts = []httperr.Option{httperr.Status(code), httperr.Message("用户不存在"), httperr.Identity()}
			}
			logger.Error("request failed", slogerr.Err(err), slog.Int("http_status", code))
			if writeErr := httperr.Write(w, err, opts...); writeErr != nil {
				logger.Error("response write failed", slogerr.Err(writeErr))
			}
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if _, err := w.Write([]byte(`{"id":` + strconv.FormatInt(id, 10) + `,"name":"alice"}`)); err != nil {
			logger.Error("response write failed", slogerr.Err(err))
		}
	}
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	mux := http.NewServeMux()
	mux.Handle("/user", handleUser(logger))
	server := &http.Server{Addr: "127.0.0.1:8080", Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	logger.Info("listening", slog.String("addr", server.Addr))
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server exited", slogerr.Err(err))
		os.Exit(1)
	}
}
