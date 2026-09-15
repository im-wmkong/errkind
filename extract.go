package errkind

// KindOf returns the primary instance's bound identity, never an arbitrary sibling's.
func KindOf(err error) *Kind {
	if d := detailsOf(err); d != nil {
		if k, ok := d.(interface{ Kind() *Kind }); ok {
			return k.Kind()
		}
	}
	return nil
}

func detailsOf(err error) Details {
	for n := 0; err != nil && n < 256; n++ {
		if d, ok := err.(Details); ok {
			return d
		}
		switch e := err.(type) {
		case interface{ Unwrap() error }:
			err = e.Unwrap()
		case interface{ Unwrap() []error }:
			var next error
			for _, child := range e.Unwrap() {
				if child == nil {
					continue
				}
				if next != nil {
					return nil
				}
				next = child
			}
			err = next
		default:
			return nil
		}
	}
	return nil
}

func CodeOf(err error) (Code, bool) {
	if d := detailsOf(err); d != nil {
		return d.BusinessCode()
	}
	return 0, false
}

func NameOf(err error) (string, bool) {
	if d := detailsOf(err); d != nil {
		name := d.Name()
		return name, name != ""
	}
	return "", false
}

func MessageOf(err error) string {
	if d := detailsOf(err); d != nil {
		return d.Message()
	}
	if err == nil {
		return ""
	}
	return err.Error()
}

func AttrsOf(err error) []Attr {
	if d := detailsOf(err); d != nil {
		return d.Attrs()
	}
	return nil
}

// AllAttrs is a lossy, depth-first flat view; the first value for each key wins.
func AllAttrs(err error) []Attr {
	var out []Attr
	seen := map[string]struct{}{}
	walk(err, func(cur error) bool {
		var attrs []Attr
		if e, ok := cur.(*kerr); ok {
			attrs = e.attrs
		} else if d, ok := cur.(Details); ok {
			attrs = d.Attrs()
		}
		for _, a := range attrs {
			if _, exists := seen[a.Key]; !exists {
				seen[a.Key] = struct{}{}
				out = append(out, a)
			}
		}
		return false
	})
	return out
}

func walk(err error, visit func(error) bool) bool {
	remaining := 256
	var walkNode func(error) bool
	walkNode = func(cur error) bool {
		for cur != nil && remaining > 0 {
			remaining--
			if visit(cur) {
				return true
			}
			switch e := cur.(type) {
			case interface{ Unwrap() error }:
				cur = e.Unwrap()
			case interface{ Unwrap() []error }:
				for _, child := range e.Unwrap() {
					if remaining == 0 {
						break
					}
					if walkNode(child) {
						return true
					}
				}
				return false
			default:
				return false
			}
		}
		return false
	}
	return walkNode(err)
}
