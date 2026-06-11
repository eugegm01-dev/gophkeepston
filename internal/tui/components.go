package tui

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/pquerna/otp/totp"

	"github.com/eugegm01-dev/gophkeepston/internal/client/authclient"
	"github.com/eugegm01-dev/gophkeepston/internal/client/crypto"
	"github.com/eugegm01-dev/gophkeepston/internal/client/session"
	"github.com/eugegm01-dev/gophkeepston/internal/client/store"
)

// Типы сообщений
type authSuccessMsg struct {
	session   *session.Session
	masterKey []byte
	store     *store.Store
}

type errMsg struct{ err error }

type entriesLoadedMsg struct {
	entries []list.Item
}

// Стили
var (
	errorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#FF0000")).
			Padding(0, 1)

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262")).
			Italic(true)

	docStyle = lipgloss.NewStyle().Margin(1, 2)
)

type screen int

const (
	screenAuth screen = iota
	screenList
	screenView
	screenAdd
)

type item struct {
	id        string
	entryType string
	title     string
	desc      string
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title }

type model struct {
	screen     screen
	authScreen authModel
	list       list.Model
	spinner    spinner.Model
	err        error
	infoMsg    string
	store      *store.Store
	session    *session.Session
	masterKey  []byte
	serverAddr string
	width      int
	height     int

	viewingEntry item
	viewingData  PasswordEntry

	addForm *huh.Form
	adding  bool
}

func NewModel(serverAddr string) *model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return &model{
		screen:     screenAuth,
		authScreen: newAuthModel(serverAddr),
		serverAddr: serverAddr,
		spinner:    s,
	}
}

func (m *model) Init() tea.Cmd {
	return m.authScreen.Init()
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.list.Items() != nil {
			m.list.SetSize(msg.Width-5, msg.Height-10)
		}
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case authSuccessMsg:
		m.session = msg.session
		m.masterKey = msg.masterKey
		m.store = msg.store
		m.screen = screenList
		return m, loadEntriesCmd(m)

	case errMsg:
		m.err = msg.err
		return m, nil

	case entriesLoadedMsg:
		items := make([]list.Item, len(msg.entries))
		copy(items, msg.entries)
		m.list = list.New(items, list.NewDefaultDelegate(), m.width-5, m.height-10)
		m.list.Title = "Entries"
		return m, nil

	case PasswordEntry:
		m.viewingData = msg
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEscape:
			// Esc на экране просмотра возвращает к списку
			if m.screen == screenView {
				m.screen = screenList
				return m, nil
			}
			// Esc на списке записей – выход из программы
			if m.screen == screenList {
				return m, tea.Quit
			}
			// На экране аутентификации и добавления Esc обработается внутри
		case tea.KeyCtrlC:
			return m, tea.Quit
		default:
			switch msg.String() {
			case "a":
				if m.screen == screenList {
					m.screen = screenAdd
					m.adding = true
					m.addForm = newAddForm()
					return m, m.addForm.Init()
				}
			case "d":
				if m.screen == screenList {
					if i, ok := m.list.SelectedItem().(item); ok {
						if err := m.store.Delete(m.session.UserID, i.id); err != nil {
							m.err = err
						} else {
							return m, loadEntriesCmd(m)
						}
					}
				}
			case "enter":
				if m.screen == screenList {
					if i, ok := m.list.SelectedItem().(item); ok {
						m.screen = screenView
						m.viewingEntry = i
						return m, viewEntryCmd(m, i.id)
					}
				}
			}
		}
	}

	var cmd tea.Cmd
	switch m.screen {
	case screenAuth:
		newAuth, authCmd := m.authScreen.Update(msg)
		m.authScreen = newAuth
		return m, authCmd
	case screenList:
		m.list, cmd = m.list.Update(msg)
	case screenAdd:
		if m.adding && m.addForm != nil {
			f, formCmd := m.addForm.Update(msg)
			if f, ok := f.(*huh.Form); ok {
				m.addForm = f
				cmd = formCmd
				if f.State == huh.StateCompleted {
					m.adding = false
					return m, saveEntryCmd(m)
				}
			}
		}
	}
	return m, cmd
}

func (m *model) View() string {
	if m.err != nil {
		return errorStyle.Render(fmt.Sprintf("Error: %v", m.err)) + "\n\nPress esc to go back."
	}
	switch m.screen {
	case screenAuth:
		return m.authScreen.View()
	case screenList:
		return docStyle.Render(m.list.View() + "\n" + m.helpView())
	case screenView:
		return m.viewEntryView()
	case screenAdd:
		if m.addForm != nil {
			return m.addForm.View()
		}
		return "Loading form..."
	}
	return ""
}

func (m *model) helpView() string {
	return infoStyle.Render("\n↑/↓: navigate • enter: view • a: add • d: delete • esc: quit")
}

func (m *model) viewEntryView() string {
	e := m.viewingData
	otpCode := ""
	if e.IsOTP && e.OTPSecret != "" {
		code, err := totp.GenerateCode(e.OTPSecret, time.Now())
		if err == nil {
			otpCode = fmt.Sprintf("\nOTP: %s (valid %d seconds)", code, 30-time.Now().Second()%30)
		}
	}
	return docStyle.Render(fmt.Sprintf(
		"Site: %s\nLogin: %s\nPassword: %s\nMeta: %s%s\n\n%s",
		e.Site, e.Login, e.Password, e.Meta, otpCode,
		infoStyle.Render("esc: back"),
	))
}

// ===== КОМАНДЫ =====

func loadEntriesCmd(m *model) tea.Cmd {
	return func() tea.Msg {
		ids, err := m.store.List(m.session.UserID)
		if err != nil {
			return errMsg{err}
		}
		var items []list.Item
		for _, id := range ids {
			ciphertext, err := m.store.Get(m.session.UserID, id)
			if err != nil {
				continue
			}
			plain, err := crypto.Decrypt(ciphertext, m.masterKey)
			if err != nil {
				continue
			}
			var entry PasswordEntry
			if err := json.Unmarshal(plain, &entry); err != nil {
				continue
			}
			title := entry.Site
			if entry.IsOTP {
				title += " (OTP)"
			}
			items = append(items, item{
				id:        id,
				entryType: "password",
				title:     title,
				desc:      entry.Login,
			})
		}
		return entriesLoadedMsg{items}
	}
}

func viewEntryCmd(m *model, id string) tea.Cmd {
	return func() tea.Msg {
		ciphertext, err := m.store.Get(m.session.UserID, id)
		if err != nil {
			return errMsg{err}
		}
		plain, err := crypto.Decrypt(ciphertext, m.masterKey)
		if err != nil {
			return errMsg{err}
		}
		var entry PasswordEntry
		if err := json.Unmarshal(plain, &entry); err != nil {
			return errMsg{err}
		}
		return entry
	}
}

func saveEntryCmd(m *model) tea.Cmd {
	return func() tea.Msg {
		form := m.addForm
		if form == nil {
			return errMsg{fmt.Errorf("form is nil")}
		}
		site := form.GetString("site")
		login := form.GetString("login")
		password := form.GetString("password")
		meta := form.GetString("meta")
		isOTP := form.GetBool("is_otp")
		otpSecret := form.GetString("otp_secret")

		if site == "" || login == "" || password == "" {
			return errMsg{fmt.Errorf("site, login and password are required")}
		}

		entry := PasswordEntry{
			Site:      site,
			Login:     login,
			Password:  password,
			Meta:      meta,
			IsOTP:     isOTP,
			OTPSecret: otpSecret,
		}
		plain, err := json.Marshal(entry)
		if err != nil {
			return errMsg{err}
		}
		ciphertext, err := crypto.Encrypt(plain, m.masterKey)
		if err != nil {
			return errMsg{err}
		}
		entryID := fmt.Sprintf("%s-%d", site, time.Now().UnixNano())
		if err := m.store.Put(m.session.UserID, entryID, ciphertext); err != nil {
			return errMsg{err}
		}
		m.screen = screenList
		return loadEntriesCmd(m)()
	}
}

// ===== ПОШАГОВАЯ АУТЕНТИФИКАЦИЯ =====

type authStep int

const (
	stepChooseAction authStep = iota
	stepEnterLogin
	stepEnterPassword
	stepAuthenticating
)

type authModel struct {
	step     authStep
	action   string
	login    string
	password string
	err      error
	server   string

	loginInput    textinput.Model
	passwordInput textinput.Model
}

func newAuthModel(server string) authModel {
	li := textinput.New()
	li.Placeholder = "Enter login"
	li.Focus()

	pi := textinput.New()
	pi.Placeholder = "Master password"
	pi.EchoMode = textinput.EchoPassword
	pi.EchoCharacter = '*'

	return authModel{
		step:          stepChooseAction,
		action:        "login",
		server:        server,
		loginInput:    li,
		passwordInput: pi,
	}
}

func (m authModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m authModel) Update(msg tea.Msg) (authModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEscape:
			// Esc на экране выбора действия — выход из программы
			if m.step == stepChooseAction {
				return m, tea.Quit
			}
			// На шагах ввода — возврат к выбору действия
			m.step = stepChooseAction
			m.err = nil
			return m, nil
		case tea.KeyCtrlC:
			return m, tea.Quit
		}
	}

	switch m.step {
	case stepChooseAction:
		return m.updateChooseAction(msg)
	case stepEnterLogin:
		return m.updateEnterLogin(msg)
	case stepEnterPassword:
		return m.updateEnterPassword(msg)
	case stepAuthenticating:
		return m, nil
	}
	return m, nil
}

func (m authModel) updateChooseAction(msg tea.Msg) (authModel, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "up", "down":
			if m.action == "login" {
				m.action = "register"
			} else {
				m.action = "login"
			}
			return m, nil
		case "enter":
			m.step = stepEnterLogin
			m.loginInput.Focus()
			return m, nil
		}
	}
	return m, nil
}

func (m authModel) updateEnterLogin(msg tea.Msg) (authModel, tea.Cmd) {
	var cmd tea.Cmd
	m.loginInput, cmd = m.loginInput.Update(msg)
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "enter" {
		m.login = m.loginInput.Value()
		if strings.TrimSpace(m.login) == "" {
			m.err = fmt.Errorf("login cannot be empty")
			return m, nil
		}
		m.step = stepEnterPassword
		m.passwordInput.Focus()
		m.err = nil
		return m, cmd
	}
	return m, cmd
}

func (m authModel) updateEnterPassword(msg tea.Msg) (authModel, tea.Cmd) {
	var cmd tea.Cmd
	m.passwordInput, cmd = m.passwordInput.Update(msg)
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "enter" {
		m.password = m.passwordInput.Value()
		if len(m.password) == 0 {
			m.err = fmt.Errorf("password cannot be empty")
			return m, nil
		}
		m.step = stepAuthenticating
		m.err = nil
		return m, m.handleAuth()
	}
	return m, cmd
}

func (m authModel) View() string {
	switch m.step {
	case stepChooseAction:
		s := "Choose action:\n\n"
		if m.action == "login" {
			s += "> Login\n  Register\n"
		} else {
			s += "  Login\n> Register\n"
		}
		s += "\n↑/↓: switch  enter: confirm  esc: quit"
		if m.err != nil {
			s = errorStyle.Render(fmt.Sprintf("Error: %v", m.err)) + "\n\n" + s
		}
		return docStyle.Render(s)
	case stepEnterLogin:
		s := "Enter login:\n\n" + m.loginInput.View()
		s += "\n\nenter: next  esc: back"
		if m.err != nil {
			s = errorStyle.Render(fmt.Sprintf("Error: %v", m.err)) + "\n\n" + s
		}
		return docStyle.Render(s)
	case stepEnterPassword:
		s := "Enter master password:\n\n" + m.passwordInput.View()
		s += "\n\nenter: login  esc: back"
		if m.err != nil {
			s = errorStyle.Render(fmt.Sprintf("Error: %v", m.err)) + "\n\n" + s
		}
		return docStyle.Render(s)
	case stepAuthenticating:
		return "Authenticating..."
	}
	return ""
}

func (m authModel) handleAuth() tea.Cmd {
	return func() tea.Msg {
		client, err := authclient.NewClient(m.server)
		if err != nil {
			return errMsg{err}
		}
		defer client.Close()

		if m.action == "register" {
			secret := make([]byte, 32)
			if _, err := rand.Read(secret); err != nil {
				return errMsg{err}
			}
			regKey := crypto.DeriveKey([]byte(m.password), []byte("gophkeepston-reg-salt"))
			encSecret, err := crypto.Encrypt(secret, regKey)
			if err != nil {
				return errMsg{err}
			}
			_, err = client.Register(context.Background(), m.login, encSecret)
			if err != nil {
				return errMsg{err}
			}
		}

		resp, err := client.Login(context.Background(), m.login)
		if err != nil {
			return errMsg{err}
		}

		regKey := crypto.DeriveKey([]byte(m.password), []byte("gophkeepston-reg-salt"))
		secret, err := crypto.Decrypt(resp.EncryptedSecret, regKey)
		if err != nil {
			return errMsg{fmt.Errorf("invalid master password")}
		}

		masterKey := crypto.DeriveKey(append(secret, []byte(m.password)...), []byte("gophkeepston-master-salt"))
		userID := m.login

		st, err := store.NewStore("gophkeepston.db")
		if err != nil {
			return errMsg{err}
		}

		sessionKey := crypto.DeriveKey([]byte(m.password), []byte("gophkeepston-session-salt"))
		if err := session.Save(sessionKey, userID, masterKey); err != nil {
			return errMsg{err}
		}

		sess := &session.Session{
			UserID:    userID,
			MasterKey: masterKey,
		}

		return authSuccessMsg{
			session:   sess,
			masterKey: masterKey,
			store:     st,
		}
	}
}

// ===== ОСТАЛЬНОЕ =====

type PasswordEntry struct {
	Site      string `json:"site"`
	Login     string `json:"login"`
	Password  string `json:"password"`
	Meta      string `json:"meta"`
	IsOTP     bool   `json:"is_otp,omitempty"`
	OTPSecret string `json:"otp_secret,omitempty"`
}

func newAddForm() *huh.Form {
	site := ""
	login := ""
	password := ""
	meta := ""
	isOTP := false
	otpSecret := ""
	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Site").Value(&site),
			huh.NewInput().Title("Login").Value(&login),
			huh.NewInput().Title("Password").EchoMode(huh.EchoModePassword).Value(&password),
			huh.NewInput().Title("Meta").Value(&meta),
			huh.NewConfirm().Title("Is it OTP?").Value(&isOTP),
			huh.NewInput().Title("OTP Secret (if OTP)").Value(&otpSecret),
		),
	).WithTheme(huh.ThemeBase())
}
