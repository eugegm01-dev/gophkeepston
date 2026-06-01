package entry

import (
	"encoding/json"
	"fmt"
)

type Base struct {
	Type Type `json:"type"`
}

func Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

func Unmarshal(data []byte) (Entry, error) {
	var probe struct {
		Type Type `json:"type"`
	}

	if err := json.Unmarshal(data, &probe); err != nil {
		return nil, fmt.Errorf("probe unmarshal: %w", err)
	}

	switch probe.Type {

	case TypePassword:
		var e PasswordEntry
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, fmt.Errorf("password unmarshal: %w", err)
		}
		return e, nil

	case TypeText:
		var e TextEntry
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, fmt.Errorf("text unmarshal: %w", err)
		}
		return e, nil
	case TypeCard: // ← добавить
		var e CardEntry
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, fmt.Errorf("card unmarshal: %w", err)
		}
		return e, nil
	case TypeBinary: // ← добавить
		var e BinaryEntry
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, fmt.Errorf("binary unmarshal: %w", err)
		}
		return e, nil
	default:
		return nil, fmt.Errorf("unknown entry type: %s", probe.Type)
	}

}
