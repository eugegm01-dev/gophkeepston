package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/eugegm01-dev/gophkeepston/internal/client/authclient"
	"github.com/eugegm01-dev/gophkeepston/internal/client/crypto"
	"github.com/eugegm01-dev/gophkeepston/internal/client/session"
	"github.com/eugegm01-dev/gophkeepston/internal/client/store"
	syncclient "github.com/eugegm01-dev/gophkeepston/internal/client/sync"
)

var (
	serverAddr string
	localStore *store.Store
	userID     string
	masterKey  []byte
)

type PasswordEntry struct {
	Site     string `json:"site"`
	Login    string `json:"login"`
	Password string `json:"password"`
	Meta     string `json:"meta"`
}

func init() {
	rootCmd.PersistentFlags().StringVar(&serverAddr, "server", "localhost:50051", "gRPC server address")
}

func requireSession(cmd *cobra.Command, args []string) error {
	// Если уже загружено (например, после login в том же процессе) – пропускаем
	if localStore != nil && masterKey != nil {
		return nil
	}
	// Пытаемся загрузить сессию
	fmt.Print("Master password: ")
	password, _ := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	s, err := session.Load(password)
	if err != nil {
		return fmt.Errorf("not logged in (session not found or invalid password)")
	}
	userID = s.UserID
	masterKey = s.MasterKey
	localStore, err = store.NewStore("gophkeepston.db")
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	if err := s.EnsureFreshAccess(serverAddr); err != nil {
		return fmt.Errorf("refresh session: %w", err)
	}
	// можно опционально синхронизироваться, но это замедлит команды
	// syncclient.FullSync(localStore, s.UserID, s.AccessToken, serverAddr)
	return nil
}

var rootCmd = &cobra.Command{
	Use:   "gophkeeper",
	Short: "GophKeeper password manager",
}

var registerCmd = &cobra.Command{
	Use:   "register",
	Short: "Register new user",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Print("Login: ")
		var login string
		fmt.Scanln(&login)
		fmt.Print("Master password: ")
		password, _ := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()

		secret := make([]byte, 32)
		if _, err := rand.Read(secret); err != nil {
			return fmt.Errorf("rand: %w", err)
		}

		regKey := crypto.DeriveKey(password, []byte("gophkeepston-reg-salt"))
		encSecret, err := crypto.Encrypt(secret, regKey)
		if err != nil {
			return fmt.Errorf("encrypt secret: %w", err)
		}

		client, err := authclient.NewClient(serverAddr)
		if err != nil {
			return err
		}
		defer client.Close()

		uid, err := client.Register(context.Background(), login, encSecret)
		if err != nil {
			return fmt.Errorf("register: %w", err)
		}
		fmt.Printf("User registered: %s\n", uid)
		return nil
	},
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login and save session",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Print("Login: ")
		var login string
		fmt.Scanln(&login)
		fmt.Print("Master password: ")
		password, _ := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()

		client, err := authclient.NewClient(serverAddr)
		if err != nil {
			return err
		}
		defer client.Close()

		resp, err := client.Login(context.Background(), login)
		if err != nil {
			return fmt.Errorf("login: %w", err)
		}

		regKey := crypto.DeriveKey(password, []byte("gophkeepston-reg-salt"))
		secret, err := crypto.Decrypt(resp.EncryptedSecret, regKey)
		if err != nil {
			return fmt.Errorf("invalid master password")
		}

		masterKey = crypto.DeriveKey(append(secret, password...), []byte("gophkeepston-master-salt"))
		userID = login

		localStore, err = store.NewStore("gophkeepston.db")
		if err != nil {
			return fmt.Errorf("open store: %w", err)
		}

		// Сохраняем сессию с токенами
		sessionKey := crypto.DeriveKey(password, []byte("gophkeepston-session-salt"))
		if err := session.Save(sessionKey, userID, masterKey, resp.AccessToken, resp.RefreshToken); err != nil {
			return fmt.Errorf("save session: %w", err)
		}

		// Синхронизируем данные с сервером
		syncclient.FullSync(localStore, userID, resp.AccessToken, serverAddr)

		fmt.Println("Logged in successfully. Session saved.")
		return nil
	},
}

var addCmd = &cobra.Command{
	Use:     "add [type]",
	Short:   "Add a new entry (password, text, card, binary)",
	Args:    cobra.ExactArgs(1),
	PreRunE: requireSession,
	RunE: func(cmd *cobra.Command, args []string) error {
		typ := args[0]
		switch typ {
		case "password":
			fmt.Print("Site: ")
			var site string
			fmt.Scanln(&site)
			fmt.Print("Login: ")
			var login string
			fmt.Scanln(&login)
			fmt.Print("Password: ")
			pass, _ := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Println()
			fmt.Print("Meta (optional): ")
			var meta string
			fmt.Scanln(&meta)

			entry := PasswordEntry{
				Site:     site,
				Login:    login,
				Password: string(pass),
				Meta:     meta,
			}
			plain, _ := json.Marshal(entry)
			ciphertext, err := crypto.Encrypt(plain, masterKey)
			if err != nil {
				return fmt.Errorf("encrypt: %w", err)
			}
			entryID := fmt.Sprintf("%s-%d", site, time.Now().UnixNano())
			if err := localStore.Put(userID, entryID, ciphertext); err != nil {
				return fmt.Errorf("store: %w", err)
			}
			fmt.Println("Entry added:", entryID)
		default:
			return fmt.Errorf("unsupported type: %s", typ)
		}
		return nil
	},
}

var getCmd = &cobra.Command{
	Use:     "get [entryID]",
	Short:   "Get and decrypt an entry",
	Args:    cobra.ExactArgs(1),
	PreRunE: requireSession,
	RunE: func(cmd *cobra.Command, args []string) error {
		entryID := args[0]
		ciphertext, err := localStore.Get(userID, entryID)
		if err != nil {
			return fmt.Errorf("get: %w", err)
		}
		plain, err := crypto.Decrypt(ciphertext, masterKey)
		if err != nil {
			return fmt.Errorf("decrypt: %w", err)
		}
		var entry PasswordEntry
		if err := json.Unmarshal(plain, &entry); err != nil {
			return fmt.Errorf("unmarshal: %w", err)
		}
		fmt.Printf("Site: %s\nLogin: %s\nPassword: %s\nMeta: %s\n",
			entry.Site, entry.Login, entry.Password, entry.Meta)
		return nil
	},
}

func main() {
	rootCmd.AddCommand(registerCmd, loginCmd, addCmd, getCmd)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
