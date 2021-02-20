package meter

import "github.com/pkg/errors"

func merge(src, dst interface{}) (interface{}, error) {
	s, err := iface2strings(src)
	if err != nil {
		return nil, err
	}
	d, err := iface2strings(dst)
	if err != nil {
		return nil, err
	}

	return append(s, d...), nil
}

func iface2strings(src interface{}) ([]string, error) {
	if src == nil {
		return []string{}, nil
	}
	switch v := src.(type) {
	case string:
		return []string{v}, nil
	case []string:
		return v, nil
	case []interface{}:
		if len(v) == 0 {
			return []string{}, nil
		}
		var strs []string
		for _, m := range v {
			if s, ok := m.(string); ok {
				strs = append(strs, s)
			} else {
				return nil, errors.Errorf("composable list accept string only, now found type %T value %v", m, m)
			}
		}
		return strs, nil
	default:
		return nil, errors.Errorf("invalid composable type %T value %v", v, v)
	}
}