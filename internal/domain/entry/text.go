package entry

type TextEntry struct {
	ID      string `json:"id"`
	Type    Type   `json:"type"`
	Title   string `json:"title"`
	Content string `json:"content"`
	Meta    string `json:"meta"`
}

func (t TextEntry) GetID() string {
	return t.ID
}

func (t TextEntry) GetType() Type {
	return TypeText
}
