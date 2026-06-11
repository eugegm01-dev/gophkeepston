package tui

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log/slog"
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

	"github.com/eugegm01-dev/gophkeepston/internal/client/authclient"
	"github.com/eugegm01-dev/gophkeepston/internal/client/crypto"
	"github.com/eugegm01-dev/gophkeepston/internal/client/session"
	"github.com/eugegm01-dev/gophkeepston/internal/client/store"
	syncclient "github.com/eugegm01-dev/gophkeepston/internal/client/sync"
	"github.com/eugegm01-dev/gophkeepston/internal/domain/entry"
	"github.com/eugegm01-dev/gophkeepston/internal/presenter"
)

var (
	errorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#FF0000")).
			Padding(0, 1)
	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			Italic(true)
	dungeonStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#555555")).
			Background(lipgloss.Color("#1a1a2e")).
			Foreground(lipgloss.Color("#c0c0c0")).
			Padding(1, 2)
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFD700")).
			Background(lipgloss.Color("#333333")).
			Padding(0, 2)
)

type authSuccessMsg struct {
	session   *session.Session
	masterKey []byte
	store     *store.Store
}
type syncCompletedMsg struct{}
type errMsg struct{ err error }
type entriesLoadedMsg struct {
	entries []list.Item
}

type item struct {
	id        string
	entryType string
	title     string
	desc      string
}

type model struct {
	screen       int
	authScreen   authModel
	list         list.Model
	spinner      spinner.Model
	err          error
	infoMsg      string
	store        *store.Store
	session      *session.Session
	masterKey    []byte
	serverAddr   string
	width        int
	height       int
	viewingEntry item
<<<<<<< feature/final-fixes-and-architecture
	viewingData  []byte
	addForm      *huh.Form
	addingType   string
	adding       bool
}

const (
	screenAuth = iota
	screenList
	screenView
	screenAdd
	screenChooseType
)

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title }
=======
	viewingData  PasswordEntry

	addForm *huh.Form
	adding  bool
}
>>>>>>> main

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
<<<<<<< feature/final-fixes-and-architecture
	case []byte:
=======

	case PasswordEntry:
>>>>>>> main
		m.viewingData = msg
		return m, nil
	case syncCompletedMsg:
		m.screen = screenList
		return m, loadEntriesCmd(m)
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
						_ = m.store.PutVersion(m.session.UserID, i.id, -1)
						return m, saveAndSyncCmd(m)
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
<<<<<<< feature/final-fixes-and-architecture
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
					var e entry.BinaryEntry
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
=======
>>>>>>> main
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
		return errorStyle.Render(fmt.Sprintf("Error: %v", m.err)) + "\nPress esc to go back."
	}
	switch m.screen {
	case screenAuth:
		return m.authScreen.View()
	case screenList:
		header := headerStyle.Render(" Vault ")
		help := m.helpView()
		content := lipgloss.JoinVertical(lipgloss.Left, header, m.list.View(), help)
		return dungeonStyle.Width(m.width - 4).Height(m.height - 4).Render(content)
	case screenView:
		header := headerStyle.Render(" Scroll ")
		body := m.viewEntryView()
		help := infoStyle.Render("esc: back")
		content := lipgloss.JoinVertical(lipgloss.Left, header, body, help)
		return dungeonStyle.Width(m.width - 4).Height(m.height - 4).Render(content)
	case screenAdd:
		if m.addForm != nil {
			header := headerStyle.Render(" Add Entry ")
			content := lipgloss.JoinVertical(lipgloss.Left, header, m.addForm.View())
			return dungeonStyle.Width(m.width - 4).Height(m.height - 4).Render(content)
		}
		return "Loading form..."
<<<<<<< feature/docs-and-polish
	case screenChooseType:
		header := headerStyle.Render(" Choose Type ")
		body := "p: password\nt: text\nc: card\nb: binary\nesc: back"
		content := lipgloss.JoinVertical(lipgloss.Left, header, body)
<<<<<<< feature/final-fixes-and-architecture
		return dungeonStyle.Width(m.width - 4).Height(m.height - 4).Render(content)
=======
		return dungeonStyle.
			Width(m.width - 4).Height(m.height - 4).
			Render(content)
=======
>>>>>>> main
>>>>>>> main
	}
	return ""
}

func (m *model) helpView() string {
	return infoStyle.Render("\n↑/↓: navigate • enter: view • a: add • d: delete • esc: quit")
}

func (m *model) viewEntryView() string {
<<<<<<< feature/docs-and-polish
	if m.viewingData == nil {
		return "No data"
	}
	e, err := entry.Unmarshal(m.viewingData)
	if err != nil {
		return fmt.Sprintf("Error parsing entry: %v", err)
	}
	view, err := presenter.Render(e, m.masterKey)
	if err != nil {
		return fmt.Sprintf("Render error: %v", err)
	}
<<<<<<< feature/final-fixes-and-architecture
	content := strings.Join(view.Lines, "\n")
	help := infoStyle.Render("\n" + strings.Join(mapToActions(view.Actions), " • "))
	return content + "\n" + help
=======
	return content
=======
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
>>>>>>> main
>>>>>>> main
}

func mapToActions(actions []presenter.Action) []string {
	var out []string
	for _, a := range actions {
		out = append(out, fmt.Sprintf("%s: %s", a.Key, a.Label))
	}
	return out
}

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
<<<<<<< feature/final-fixes-and-architecture
			var typeCheck struct {
				Type string `json:"type"`
			}
			if err := json.Unmarshal(plain, &typeCheck); err != nil {
=======
			var entry PasswordEntry
			if err := json.Unmarshal(plain, &entry); err != nil {
>>>>>>> main
				continue
			}
<<<<<<< feature/final-fixes
			ver, _ := m.store.GetVersion(m.session.UserID, id)
			if ver == -1 {
				continue
			}
			switch typeCheck.Type {
			case "password":
				var e entry.PasswordEntry
				if err := json.Unmarshal(plain, &e); err != nil {
					continue
				}
				title := e.Site
				if e.IsOTP {
					title += " (OTP)"
				}
				items = append(items, item{id: id, entryType: "password", title: title, desc: e.Login})
			case "text":
				var e entry.TextEntry
				if err := json.Unmarshal(plain, &e); err != nil {
					continue
				}
				preview := e.Content
				if len(preview) > 50 {
					preview = preview[:50] + "..."
				}
				items = append(items, item{id: id, entryType: "text", title: e.Title, desc: preview})
			case "card":
				var e entry.CardEntry
				if err := json.Unmarshal(plain, &e); err != nil {
					continue
				}
				items = append(items, item{id: id, entryType: "card", title: e.Holder, desc: e.Number + " " + e.Expiry})
			case "binary":
				var e entry.BinaryEntry
				if err := json.Unmarshal(plain, &e); err != nil {
					continue
				}
<<<<<<< feature/final-fixes-and-architecture
				items = append(items, item{id: id, entryType: "binary", title: e.FileName, desc: fmt.Sprintf("%d bytes", len(e.Data))})
			}
=======
				items = append(items, item{
					id:        id,
					entryType: "binary",
					title:     entry.FileName,
					desc:      fmt.Sprintf("%d bytes", len(entry.Data)),
				})
=======
			title := entry.Site
			if entry.IsOTP {
				title += " (OTP)"
>>>>>>> main
			}
			items = append(items, item{
				id:        id,
				entryType: "password",
				title:     title,
				desc:      entry.Login,
			})
>>>>>>> main
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
<<<<<<< feature/docs-and-polish
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
			plain, err := json.Marshal(entry.PasswordEntry{Type: entry.TypePassword, Site: site, Login: login, Password: password, Meta: meta, IsOTP: isOTP, OTPSecret: otpSecret})
			if err != nil {
				return errMsg{fmt.Errorf("marshal %s entry: %w", m.addingType, err)}
			}
			ciphertext, err := crypto.Encrypt(plain, m.masterKey)
			if err != nil {
				return errMsg{err}
			}
			entryID := fmt.Sprintf("%s-%d", site, time.Now().UnixNano())
			if err := m.store.Put(m.session.UserID, entryID, ciphertext); err != nil {
				return errMsg{err}
			}
			_ = m.store.PutVersion(m.session.UserID, entryID, time.Now().UnixNano())
		case "text":
			title := form.GetString("title")
			content := form.GetString("content")
			meta := form.GetString("meta")
			if title == "" || content == "" {
				return errMsg{fmt.Errorf("title and content are required")}
			}
			plain, _ := json.Marshal(entry.TextEntry{Type: entry.TypeText, Title: title, Content: content, Meta: meta})
			ciphertext, err := crypto.Encrypt(plain, m.masterKey)
			if err != nil {
				return errMsg{fmt.Errorf("marshal %s entry: %w", m.addingType, err)}
			}
			entryID := fmt.Sprintf("text-%d", time.Now().UnixNano())
			if err := m.store.Put(m.session.UserID, entryID, ciphertext); err != nil {
				return errMsg{err}
			}
			_ = m.store.PutVersion(m.session.UserID, entryID, time.Now().UnixNano())
		case "card":
			number := form.GetString("number")
			expiry := form.GetString("expiry")
			cvv := form.GetString("cvv")
			holder := form.GetString("holder")
			meta := form.GetString("meta")
			if number == "" || expiry == "" || cvv == "" {
				return errMsg{fmt.Errorf("number, expiry and cvv are required")}
			}
			plain, _ := json.Marshal(entry.CardEntry{Type: entry.TypeCard, Number: number, Expiry: expiry, CVV: cvv, Holder: holder, Meta: meta})
			ciphertext, err := crypto.Encrypt(plain, m.masterKey)
			if err != nil {
				return errMsg{fmt.Errorf("marshal %s entry: %w", m.addingType, err)}
			}
			entryID := fmt.Sprintf("card-%d", time.Now().UnixNano())
			if err := m.store.Put(m.session.UserID, entryID, ciphertext); err != nil {
				return errMsg{err}
			}
			_ = m.store.PutVersion(m.session.UserID, entryID, time.Now().UnixNano())
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
			plain, _ := json.Marshal(entry.BinaryEntry{Type: entry.TypeBinary, FileName: fileName, Data: data, Meta: meta})
			ciphertext, err := crypto.Encrypt(plain, m.masterKey)
			if err != nil {
				return errMsg{err}
			}
			entryID := fmt.Sprintf("binary-%d", time.Now().UnixNano())
			if err := m.store.Put(m.session.UserID, entryID, ciphertext); err != nil {
				return errMsg{err}
			}
<<<<<<< feature/final-fixes-and-architecture
			_ = m.store.PutVersion(m.session.UserID, entryID, time.Now().UnixNano())
		}
=======
			ver := time.Now().UnixNano()
			_ = m.store.PutVersion(m.session.UserID, entryID, ver)

=======
		site := form.GetString("site")
		login := form.GetString("login")
		password := form.GetString("password")
		meta := form.GetString("meta")
		isOTP := form.GetBool("is_otp")
		otpSecret := form.GetString("otp_secret")

		if site == "" || login == "" || password == "" {
			return errMsg{fmt.Errorf("site, login and password are required")}
>>>>>>> main
		}
<<<<<<< feature/final-fixes
		m.spinner, _ = m.spinner.Update(spinner.TickMsg{})
>>>>>>> main
		return tea.Batch(
			func() tea.Msg {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				_ = syncclient.FullSync(ctx, m.store, m.session.UserID, m.session.AccessToken, m.serverAddr)
				return syncCompletedMsg{}
			},
		)
=======
		go func() {
			_ = syncclient.FullSync(m.store, m.session.UserID, m.session.AccessToken, m.serverAddr)
		}()

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
>>>>>>> main
	}
}

func saveAndSyncCmd(m *model) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		syncclient.FullSync(ctx, m.store, m.session.UserID, m.session.AccessToken, m.serverAddr)
		return syncCompletedMsg{}
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
		return m, m.handleAuth(m.server)
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
func (m authModel) handleAuth(serverAddr string) tea.Cmd {
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
			regKey, err := crypto.DeriveKey([]byte(m.password), []byte("gophkeepston-reg-salt"))
			if err != nil {
				return errMsg{fmt.Errorf("derive reg key: %w", err)}
			}
			encSecret, err := crypto.Encrypt(secret, regKey)
			if err != nil {
				return errMsg{err}
			}
			_, err = client.Register(context.Background(), m.login, encSecret)
			if err != nil {
				return errMsg{err}
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		resp, err := client.Login(ctx, m.login)
		if err != nil {
			return errMsg{fmt.Errorf("login failed: %w", err)}
		}
		regKey, err := crypto.DeriveKey([]byte(m.password), []byte("gophkeepston-reg-salt"))
		if err != nil {
			return errMsg{fmt.Errorf("derive reg key: %w", err)}
		}
		secret, err := crypto.Decrypt(resp.EncryptedSecret, regKey)
		if err != nil {
			return errMsg{fmt.Errorf("invalid master password")}
		}

		masterKey, err := crypto.DeriveKey(append(secret, []byte(m.password)...), resp.Salt)

		if err != nil {
			return errMsg{fmt.Errorf("derive master key: %w", err)}
		}
		userID := m.login

		st, err := store.NewStore("gophkeepston.db")
		if err != nil {
			return errMsg{err}
		}

<<<<<<< feature/final-fixes-and-architecture
		// Сохраняем сессию с токенами
		sessionKey, err := crypto.DeriveKey(append(secret, []byte(m.password)...), resp.Salt)
		if err != nil {
			return errMsg{fmt.Errorf("derive session key: %w", err)}
		}
		if err := session.Save(sessionKey, userID, masterKey, resp.AccessToken, resp.RefreshToken); err != nil {
			return errMsg{err}
		}

		// Синхронизация с сервером

		syncCtx, syncCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer syncCancel()
		if err := syncclient.FullSync(syncCtx, st, userID, resp.AccessToken, serverAddr); err != nil {
			slog.Warn("initial sync failed", "error", err)
		}
=======
		sessionKey := crypto.DeriveKey([]byte(m.password), []byte("gophkeepston-session-salt"))
		if err := session.Save(sessionKey, userID, masterKey); err != nil {
			return errMsg{err}
		}

>>>>>>> main
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

<<<<<<< feature/final-fixes-and-architecture
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
=======
type PasswordEntry struct {
	Site      string `json:"site"`
	Login     string `json:"login"`
	Password  string `json:"password"`
	Meta      string `json:"meta"`
	IsOTP     bool   `json:"is_otp,omitempty"`
	OTPSecret string `json:"otp_secret,omitempty"`
>>>>>>> main
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
			huh.NewInput().Key("site").Title("Site").Value(&site),
			huh.NewInput().Key("login").Title("Login").Value(&login),
			huh.NewInput().Key("password").Title("Password").EchoMode(huh.EchoModePassword).Value(&password),
			huh.NewInput().Key("meta").Title("Meta").Value(&meta),
			huh.NewConfirm().Key("is_otp").Title("Is it OTP?").Value(&isOTP),
			huh.NewInput().Key("otp_secret").Title("OTP Secret (if OTP)").Value(&otpSecret),
		),
	).WithTheme(huh.ThemeBase())
}
<<<<<<< feature/docs-and-polish

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
=======
>>>>>>> main
