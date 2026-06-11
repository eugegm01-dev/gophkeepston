package entry

type Type string

const (
	TypePassword Type = "password"
	TypeText     Type = "text"
	TypeCard     Type = "card"
	TypeBinary   Type = "binary"
)

type Entry interface {
	GetID() string
	GetType() Type
}
