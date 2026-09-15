package errkind

type Option func(*kerr)

func With(key string, val any) Option {
	return func(e *kerr) {
		for i := range e.attrs {
			if e.attrs[i].Key == key {
				e.attrs[i].Val = val
				return
			}
		}
		e.attrs = append(e.attrs, Attr{Key: key, Val: val})
	}
}
