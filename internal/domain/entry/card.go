package entry

type CardEntry struct {
	ID     string `json:"id"`
	Type   Type   `json:"type"`
	Number string `json:"number"`
	Expiry string `json:"expiry"`
	CVV    string `json:"cvv"`
	Holder string `json:"holder"`
	Meta   string `json:"meta"`
}

func (c CardEntry) GetID() string { return c.ID }
func (c CardEntry) GetType() Type { return TypeCard }
