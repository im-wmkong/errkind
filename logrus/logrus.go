// Package logrus provides explicit structured error logging for logrus.
package logrus

import (
	"github.com/im-wmkong/errkind/internal/diagnostic"
	"github.com/sirupsen/logrus"
)

func Fields(err error) logrus.Fields { return FieldsWithPrefix("err", err) }

func FieldsWithPrefix(prefix string, err error) logrus.Fields {
	return logrus.Fields{prefix: diagnostic.Build(err)}
}
