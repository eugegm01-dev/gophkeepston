// internal/presenter/presenter.go
package presenter

import (
	"fmt"
	"time"

	"github.com/eugegm01-dev/gophkeepston/internal/domain/entry"
	"github.com/pquerna/otp/totp"
)

type EntryView struct {
	ID      string
	Type    string
	Title   string
	Lines   []string // отформатированные строки для отображения
	Actions []Action // доступные действия
}

type Action struct {
	Key   string
	Label string
}

func Render(e entry.Entry, masterKey []byte) (*EntryView, error) {
	view := &EntryView{
		ID:   e.GetID(),
		Type: string(e.GetType()),
	}

	switch v := e.(type) {
	case entry.PasswordEntry:
		view.Title = v.Site
		view.Lines = []string{
			fmt.Sprintf("Site: %s", v.Site),
			fmt.Sprintf("Login: %s", v.Login),
			fmt.Sprintf("Password: %s", v.Password),
		}
		if v.Meta != "" {
			view.Lines = append(view.Lines, fmt.Sprintf("Meta: %s", v.Meta))
		}
		if v.IsOTP && v.OTPSecret != "" {
			if code, err := totp.GenerateCode(v.OTPSecret, time.Now()); err == nil {
				view.Lines = append(view.Lines, fmt.Sprintf("OTP: %s (valid %ds)", code, 30-time.Now().Second()%30))
			}
		}
		view.Actions = []Action{{Key: "c", Label: "Copy password"}}

	case entry.TextEntry:
		view.Title = v.Title
		view.Lines = []string{
			fmt.Sprintf("Title: %s", v.Title),
			fmt.Sprintf("Content: %s", v.Content),
		}
		if v.Meta != "" {
			view.Lines = append(view.Lines, fmt.Sprintf("Meta: %s", v.Meta))
		}
		view.Actions = []Action{{Key: "c", Label: "Copy content"}}

	case entry.CardEntry:
		view.Title = v.Holder
		view.Lines = []string{
			fmt.Sprintf("Number: %s", v.Number),
			fmt.Sprintf("Expiry: %s", v.Expiry),
			fmt.Sprintf("CVV: %s", v.CVV),
			fmt.Sprintf("Holder: %s", v.Holder),
		}
		if v.Meta != "" {
			view.Lines = append(view.Lines, fmt.Sprintf("Meta: %s", v.Meta))
		}
		view.Actions = []Action{{Key: "c", Label: "Copy number"}}

	case entry.BinaryEntry:
		view.Title = v.FileName
		view.Lines = []string{
			fmt.Sprintf("File: %s", v.FileName),
			fmt.Sprintf("Size: %d bytes", len(v.Data)),
		}
		if v.Meta != "" {
			view.Lines = append(view.Lines, fmt.Sprintf("Meta: %s", v.Meta))
		}
		view.Actions = []Action{{Key: "s", Label: "Save to file"}}
	}

	return view, nil
}
