package entry

type PasswordEntry struct {
	ID        string `json:"id"`
	Type      Type   `json:"type"`
	Site      string `json:"site"`
	Login     string `json:"login"`
	Password  string `json:"password"`
	Meta      string `json:"meta"`
	IsOTP     bool   `json:"is_otp,omitempty"`
	OTPSecret string `json:"otp_secret,omitempty"`
}

func (p PasswordEntry) GetID() string {
	return p.ID
}

func (p PasswordEntry) GetType() Type {
	return TypePassword
}
