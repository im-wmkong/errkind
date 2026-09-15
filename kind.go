package errkind

import "strconv"

type Kind struct {
	code     Code
	name     string
	registry *Registry
}

func (k *Kind) Code() Code   { return k.code }
func (k *Kind) Name() string { return k.name }
func (k *Kind) Error() string {
	return k.name + "(" + strconv.FormatUint(uint64(k.code), 10) + ")"
}

func (k *Kind) New(msg string, opts ...Option) error {
	return k.build(nil, msg, opts)
}

func (k *Kind) Wrap(cause error, msg string, opts ...Option) error {
	if cause == nil {
		return nil
	}
	return k.build(cause, msg, opts)
}

func (k *Kind) build(cause error, msg string, opts []Option) error {
	e := &kerr{kind: k, message: msg, cause: cause}
	for _, o := range opts {
		o(e)
	}
	if k.registry.captureStack && !hasStack(cause) {
		e.pcs = capturePCs(4)
	}
	return e
}
