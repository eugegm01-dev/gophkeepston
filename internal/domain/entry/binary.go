package entry

type BinaryEntry struct {
	ID       string `json:"id"`
	Type     Type   `json:"type"`
	FileName string `json:"file_name"`
	Data     []byte `json:"data"`
	Meta     string `json:"meta"`
}

func (b BinaryEntry) GetID() string { return b.ID }
func (b BinaryEntry) GetType() Type { return TypeBinary }
