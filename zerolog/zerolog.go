// Package zerolog provides explicit structured error logging for zerolog.
package zerolog

import (
	"github.com/im-wmkong/errkind/internal/diagnostic"
	"github.com/rs/zerolog"
)

func Err(err error) func(*zerolog.Event) { return Field("err", err) }

func Field(key string, err error) func(*zerolog.Event) {
	return func(event *zerolog.Event) {
		event.RawJSON(key, diagnostic.Encode(err))
	}
}
