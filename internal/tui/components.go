package tui

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/term"
	"github.com/pquerna/otp/totp"

	"github.com/eugegm01-dev/gophkeepston/internal/client/authclient"
	"github.com/eugegm01-dev/gophkeepston/internal/client/crypto"
	"github.com/eugegm01-dev/gophkeepston/internal/client/session"
	"github.com/eugegm01-dev/gophkeepston/internal/client/store"
	syncclient "github.com/eugegm01-dev/gophkeepston/internal/client/sync"
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
			Foreground(lipgloss.Color("#888888")).
			Italic(true)

	// Основной стиль dungeon – тёмный фон с каменной рамкой
	dungeonStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#555555")).
			Background(lipgloss.Color("#1a1a2e")).
			Foreground(lipgloss.Color("#c0c0c0")).
			Padding(1, 2)

	// Заголовок внутри dungeon
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFD700")).
			Background(lipgloss.Color("#333333")).
			Padding(0, 2)

	torch  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFA500"))
	stone  = lipgloss.NewStyle().Foreground(lipgloss.Color("#808080"))
	shadow = lipgloss.NewStyle().Foreground(lipgloss.Color("#333333"))
)

type screen int

const (
	screenAuth screen = iota
	screenList
	screenView
	screenAdd
	screenChooseType
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
	viewingData  []byte

	addForm    *huh.Form
	addingType string
	adding     bool
}

type BinaryEntry struct {
	Type     string `json:"type"` // "binary"
	FileName string `json:"file_name"`
	Data     []byte `json:"data"`
	Meta     string `json:"meta"`
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
			m.list.SetSize(msg.Width-8, msg.Height-14)
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
		m.list = list.New(items, list.NewDefaultDelegate(), m.width-8, m.height-14)
		m.list.Title = ""
		return m, nil

	case []byte:
		m.viewingData = msg
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEscape:
			if m.screen == screenView {
				m.screen = screenList
				return m, nil
			}
			if m.screen == screenList {
				return m, tea.Quit
			}
		case tea.KeyCtrlC:
			return m, tea.Quit
		default:
			switch msg.String() {
			case "a":
				if m.screen == screenList {
					m.screen = screenChooseType
					return m, nil
				}
			case "d":
				if m.screen == screenList {
					if i, ok := m.list.SelectedItem().(item); ok {
						if err := m.store.Delete(m.session.UserID, i.id); err != nil {
							m.err = err
						} else {
							_ = m.store.DeleteVersion(m.session.UserID, i.id)
							go func() {
								_ = syncclient.FullSync(m.store, m.session.UserID, m.session.AccessToken, m.serverAddr)
							}()
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
			case "p":
				if m.screen == screenChooseType {
					m.addingType = "password"
					m.screen = screenAdd
					m.adding = true
					m.addForm = newAddForm()
					return m, m.addForm.Init()
				}
			case "t":
				if m.screen == screenChooseType {
					m.addingType = "text"
					m.screen = screenAdd
					m.adding = true
					m.addForm = newTextForm()
					return m, m.addForm.Init()
				}
			case "c":
				if m.screen == screenChooseType {
					m.addingType = "card"
					m.screen = screenAdd
					m.adding = true
					m.addForm = newCardForm()
					return m, m.addForm.Init()
				}
			case "b":
				if m.screen == screenChooseType {
					m.addingType = "binary"
					m.screen = screenAdd
					m.adding = true
					m.addForm = newBinaryForm()
					return m, m.addForm.Init()
				}
			case "s":
				if m.screen == screenView && m.viewingEntry.entryType == "binary" {
					var e BinaryEntry
					if err := json.Unmarshal(m.viewingData, &e); err == nil {
						outPath := e.FileName
						os.WriteFile(outPath, e.Data, 0644)
						m.err = fmt.Errorf("file saved to %s", outPath)
					}
				}
			case "esc":
				if m.screen == screenChooseType {
					m.screen = screenList
					return m, nil
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
		return renderCastle() + "\n" + m.authScreen.View()
	case screenList:
		header := headerStyle.Render(" Vault ")
		help := m.helpView()
		content := lipgloss.JoinVertical(lipgloss.Left,
			header,
			m.list.View(),
			help,
		)
		return dungeonStyle.
			Width(m.width - 4).Height(m.height - 4).
			Render(content)
	case screenView:
		header := headerStyle.Render(" Scroll ")
		body := m.viewEntryView()
		help := infoStyle.Render("esc: back")
		content := lipgloss.JoinVertical(lipgloss.Left, header, body, help)
		return dungeonStyle.
			Width(m.width - 4).Height(m.height - 4).
			Render(content)
	case screenAdd:
		if m.addForm != nil {
			header := headerStyle.Render(" Add Entry ")
			content := lipgloss.JoinVertical(lipgloss.Left, header, m.addForm.View())
			return dungeonStyle.
				Width(m.width - 4).Height(m.height - 4).
				Render(content)
		}
		return "Loading form..."
	case screenChooseType:
		header := headerStyle.Render(" Choose Type ")
		body := "p: password\nt: text\nc: card\nb: binary\n\nesc: back"
		content := lipgloss.JoinVertical(lipgloss.Left, header, body)
		return dungeonStyle.
			Width(m.width - 4).Height(m.height - 4).
			Render(content)
	}
	return ""
}

func (m *model) helpView() string {
	return infoStyle.Render("\n↑/↓: navigate • enter: view • a: add • d: delete • esc: quit")
}

func (m *model) viewEntryView() string {
	if m.viewingData == nil {
		return "No data"
	}
	var typeCheck struct{ Type string }
	if err := json.Unmarshal(m.viewingData, &typeCheck); err != nil {
		return "Error parsing entry"
	}
	var content string
	switch typeCheck.Type {
	case "password":
		var e PasswordEntry
		if err := json.Unmarshal(m.viewingData, &e); err == nil {
			otpCode := ""
			if e.IsOTP && e.OTPSecret != "" {
				code, err := totp.GenerateCode(e.OTPSecret, time.Now())
				if err == nil {
					otpCode = fmt.Sprintf("\nOTP: %s (valid %d seconds)", code, 30-time.Now().Second()%30)
				}
			}
			content = fmt.Sprintf("Site: %s\nLogin: %s\nPassword: %s\nMeta: %s%s",
				e.Site, e.Login, e.Password, e.Meta, otpCode)
		}
	case "text":
		var e TextEntry
		if err := json.Unmarshal(m.viewingData, &e); err == nil {
			content = fmt.Sprintf("Title: %s\nContent: %s\nMeta: %s", e.Title, e.Content, e.Meta)
		}
	case "card":
		var e CardEntry
		if err := json.Unmarshal(m.viewingData, &e); err == nil {
			content = fmt.Sprintf("Number: %s\nExpiry: %s\nCVV: %s\nHolder: %s\nMeta: %s",
				e.Number, e.Expiry, e.CVV, e.Holder, e.Meta)
		}
	case "binary":
		var e BinaryEntry
		if err := json.Unmarshal(m.viewingData, &e); err == nil {
			content = fmt.Sprintf("File: %s\nSize: %d bytes\nMeta: %s",
				e.FileName, len(e.Data), e.Meta)
		}
	default:
		content = "Unknown entry type"
	}
	return content
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

			var typeCheck struct {
				Type string `json:"type"`
			}
			if err := json.Unmarshal(plain, &typeCheck); err != nil {
				continue
			}

			switch typeCheck.Type {
			case "password":
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
			case "text":
				var entry TextEntry
				if err := json.Unmarshal(plain, &entry); err != nil {
					continue
				}
				preview := entry.Content
				if len(preview) > 50 {
					preview = preview[:50] + "..."
				}
				items = append(items, item{
					id:        id,
					entryType: "text",
					title:     entry.Title,
					desc:      preview,
				})
			case "card":
				var entry CardEntry
				if err := json.Unmarshal(plain, &entry); err != nil {
					continue
				}
				items = append(items, item{
					id:        id,
					entryType: "card",
					title:     entry.Holder,
					desc:      entry.Number + " " + entry.Expiry,
				})
			case "binary":
				var entry BinaryEntry
				if err := json.Unmarshal(plain, &entry); err != nil {
					continue
				}
				items = append(items, item{
					id:        id,
					entryType: "binary",
					title:     entry.FileName,
					desc:      fmt.Sprintf("%d bytes", len(entry.Data)),
				})
			}
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
		return plain
	}
}

func saveEntryCmd(m *model) tea.Cmd {
	return func() tea.Msg {
		form := m.addForm
		if form == nil {
			return errMsg{fmt.Errorf("form is nil")}
		}
		switch m.addingType {
		case "password":
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
				Type:      "password",
				Site:      site,
				Login:     login,
				Password:  password,
				Meta:      meta,
				IsOTP:     isOTP,
				OTPSecret: otpSecret,
			}
			plain, _ := json.Marshal(entry)
			ciphertext, err := crypto.Encrypt(plain, m.masterKey)
			if err != nil {
				return errMsg{err}
			}
			entryID := fmt.Sprintf("%s-%d", site, time.Now().UnixNano())
			if err := m.store.Put(m.session.UserID, entryID, ciphertext); err != nil {
				return errMsg{err}
			}
			ver := time.Now().UnixNano()
			_ = m.store.PutVersion(m.session.UserID, entryID, ver)
		case "text":
			title := form.GetString("title")
			content := form.GetString("content")
			meta := form.GetString("meta")

			if title == "" || content == "" {
				return errMsg{fmt.Errorf("title and content are required")}
			}
			entry := TextEntry{
				Type:    "text",
				Title:   title,
				Content: content,
				Meta:    meta,
			}
			plain, _ := json.Marshal(entry)
			ciphertext, err := crypto.Encrypt(plain, m.masterKey)
			if err != nil {
				return errMsg{err}
			}
			entryID := fmt.Sprintf("text-%d", time.Now().UnixNano())
			if err := m.store.Put(m.session.UserID, entryID, ciphertext); err != nil {
				return errMsg{err}
			}
			ver := time.Now().UnixNano()
			_ = m.store.PutVersion(m.session.UserID, entryID, ver)
		case "card":
			number := form.GetString("number")
			expiry := form.GetString("expiry")
			cvv := form.GetString("cvv")
			holder := form.GetString("holder")
			meta := form.GetString("meta")

			if number == "" || expiry == "" || cvv == "" {
				return errMsg{fmt.Errorf("number, expiry and cvv are required")}
			}
			entry := CardEntry{
				Type:   "card",
				Number: number,
				Expiry: expiry,
				CVV:    cvv,
				Holder: holder,
				Meta:   meta,
			}
			plain, _ := json.Marshal(entry)
			ciphertext, err := crypto.Encrypt(plain, m.masterKey)
			if err != nil {
				return errMsg{err}
			}
			entryID := fmt.Sprintf("card-%d", time.Now().UnixNano())
			if err := m.store.Put(m.session.UserID, entryID, ciphertext); err != nil {
				return errMsg{err}
			}
			ver := time.Now().UnixNano()
			_ = m.store.PutVersion(m.session.UserID, entryID, ver)
		case "binary":
			path := form.GetString("path")
			meta := form.GetString("meta")
			if path == "" {
				return errMsg{fmt.Errorf("file path required")}
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return errMsg{err}
			}
			fileName := filepath.Base(path)
			entry := BinaryEntry{
				Type:     "binary",
				FileName: fileName,
				Data:     data,
				Meta:     meta,
			}
			plain, _ := json.Marshal(entry)
			ciphertext, err := crypto.Encrypt(plain, m.masterKey)
			if err != nil {
				return errMsg{err}
			}
			entryID := fmt.Sprintf("binary-%d", time.Now().UnixNano())
			if err := m.store.Put(m.session.UserID, entryID, ciphertext); err != nil {
				return errMsg{err}
			}
			ver := time.Now().UnixNano()
			_ = m.store.PutVersion(m.session.UserID, entryID, ver)

		}

		go func() {
			_ = syncclient.FullSync(m.store, m.session.UserID, m.session.AccessToken, m.serverAddr)
		}()

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
			if m.step == stepChooseAction {
				return m, tea.Quit
			}
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
		return s
	case stepEnterLogin:
		s := "Enter login:\n\n" + m.loginInput.View()
		s += "\n\nenter: next  esc: back"
		if m.err != nil {
			s = errorStyle.Render(fmt.Sprintf("Error: %v", m.err)) + "\n\n" + s
		}
		return s
	case stepEnterPassword:
		s := "Enter master password:\n\n" + m.passwordInput.View()
		s += "\n\nenter: login  esc: back"
		if m.err != nil {
			s = errorStyle.Render(fmt.Sprintf("Error: %v", m.err)) + "\n\n" + s
		}
		return s
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

		// Сохраняем сессию с токенами
		sessionKey := crypto.DeriveKey([]byte(m.password), []byte("gophkeepston-session-salt"))
		if err := session.Save(sessionKey, userID, masterKey, resp.AccessToken, resp.RefreshToken); err != nil {
			return errMsg{err}
		}

		// Синхронизация с сервером
		syncclient.FullSync(st, userID, resp.AccessToken, m.server)

		sess := &session.Session{
			UserID:       userID,
			MasterKey:    masterKey,
			AccessToken:  resp.AccessToken,
			RefreshToken: resp.RefreshToken,
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
	Type      string `json:"type"`
	Site      string `json:"site"`
	Login     string `json:"login"`
	Password  string `json:"password"`
	Meta      string `json:"meta"`
	IsOTP     bool   `json:"is_otp,omitempty"`
	OTPSecret string `json:"otp_secret,omitempty"`
}

type TextEntry struct {
	Type    string `json:"type"`
	Title   string `json:"title"`
	Content string `json:"content"`
	Meta    string `json:"meta"`
}

type CardEntry struct {
	Type   string `json:"type"`
	Number string `json:"number"`
	Expiry string `json:"expiry"`
	CVV    string `json:"cvv"`
	Holder string `json:"holder"`
	Meta   string `json:"meta"`
}

func newTextForm() *huh.Form {
	title := ""
	content := ""
	meta := ""
	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Key("title").Title("Title").Value(&title),
			huh.NewInput().Key("content").Title("Content").Value(&content),
			huh.NewInput().Key("meta").Title("Meta (optional)").Value(&meta),
		),
	).WithTheme(huh.ThemeBase())
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

func newCardForm() *huh.Form {
	number := ""
	expiry := ""
	cvv := ""
	holder := ""
	meta := ""
	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Key("number").
				Title("Card number (16 digits)").
				Value(&number).
				Validate(func(s string) error {
					if len(s) != 16 {
						return fmt.Errorf("must be 16 digits")
					}
					for _, c := range s {
						if c < '0' || c > '9' {
							return fmt.Errorf("only digits allowed")
						}
					}
					return nil
				}),
			huh.NewInput().
				Key("expiry").
				Title("Expiry (MM/YY)").
				Value(&expiry).
				Validate(func(s string) error {
					if len(s) != 5 || s[2] != '/' {
						return fmt.Errorf("format MM/YY")
					}
					return nil
				}),
			huh.NewInput().
				Key("cvv").
				Title("CVV (3 digits)").
				EchoMode(huh.EchoModePassword).
				Value(&cvv).
				Validate(func(s string) error {
					if len(s) != 3 {
						return fmt.Errorf("must be 3 digits")
					}
					for _, c := range s {
						if c < '0' || c > '9' {
							return fmt.Errorf("only digits")
						}
					}
					return nil
				}),
			huh.NewInput().Key("holder").Title("Holder name").Value(&holder),
			huh.NewInput().Key("meta").Title("Meta (optional)").Value(&meta),
		),
	).WithTheme(huh.ThemeBase())
}

func newBinaryForm() *huh.Form {
	path := ""
	meta := ""
	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Key("path").Title("File path").Value(&path),
			huh.NewInput().Key("meta").Title("Meta (optional)").Value(&meta),
		),
	).WithTheme(huh.ThemeBase())
}

func renderCastle() string {
	sky := lipgloss.NewStyle().Background(lipgloss.Color("#1E90FF"))
	sun := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFD700"))
	stone := lipgloss.NewStyle().Foreground(lipgloss.Color("#808080"))
	dragon := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5555"))
	knight := lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00"))
	gold := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFD700"))
	grass := lipgloss.NewStyle().Foreground(lipgloss.Color("#228B22"))

	// Ширина замка 80 символов, адаптируем под любой терминал
	width := 80
	if termWidth, _, err := term.GetSize(0); err == nil {
		if termWidth > 0 {
			width = termWidth
		}
	}

	// Сцена с рыцарем, драконом и замком
	scene := lipgloss.JoinVertical(lipgloss.Left,
		sky.Render(strings.Repeat(" ", width)),
		sky.Render("                     "+sun.Render("  \\   /  ")+"                                    "),
		sky.Render("                      "+sun.Render(".-- ☀ --.")+"                                    "),
		sky.Render("                     "+sun.Render("  /   \\  ")+"                                    "),
		sky.Render(strings.Repeat(" ", width)),
		stone.Render("        ▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄        "),
		stone.Render("      ██  ██  ██  "+knight.Render(" ██  ██  ")+stone.Render("██  ██  ██  ██  ")),
		stone.Render("      ██  ██  ██  "+knight.Render(" ██  ██  ")+stone.Render("██  ██  ██  ██  ")),
		stone.Render("      ██  ██  ██                  ██  ██  ██  ██  "),
		stone.Render("   ▄▄▄██▄▄██▄▄██▄▄▄▄▄▄▄▄▄▄▄▄▄▄██▄▄██▄▄██▄▄██   "),
		stone.Render("   ██████████████████████████████████████████████   "),
		gold.Render("   ███  ███  ██████████████████  ███  ███  ███   "),
		gold.Render("   ███  ███  ██████████████████  ███  ███  ███   "),
		dragon.Render("   ███  ███  ██████████████████  ███  ███  ███   "),
		grass.Render("~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~"),
	)

	return lipgloss.NewStyle().MaxWidth(width).Render(scene)
}
