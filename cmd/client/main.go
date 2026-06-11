package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"golang.org/x/term"

	"github.com/eugegm01-dev/gophkeepston/internal/client/authclient"
	"github.com/eugegm01-dev/gophkeepston/internal/client/crypto"
	"github.com/eugegm01-dev/gophkeepston/internal/client/session"
	"github.com/eugegm01-dev/gophkeepston/internal/client/store"
	syncclient "github.com/eugegm01-dev/gophkeepston/internal/client/sync"
	domainentry "github.com/eugegm01-dev/gophkeepston/internal/domain/entry"
)

var (
	buildVersion = "N/A"
	buildDate    = "N/A"
)

var (
	serverAddr  string
	localStore  *store.Store
	userID      string
	masterKey   []byte
	accessToken string
)

func init() {
	rootCmd.PersistentFlags().StringVar(&serverAddr, "server", "localhost:50051", "gRPC server address")
}

func requireSession(cmd *cobra.Command, args []string) error {
	if localStore != nil && masterKey != nil {
		return nil
	}
	fmt.Print("Master password: ")
	password, _ := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	s, err := session.Load(password)
	if err != nil {
		return fmt.Errorf("not logged in (session not found or invalid password)")
	}
	userID = s.UserID
	masterKey = s.MasterKey
	// Не присваиваем accessToken сразу, а возьмём после возможного refresh
	localStore, err = store.NewStore("gophkeepston.db")
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}

	sessionKey, err := crypto.DeriveKey(password, []byte("gophkeepston-session-salt"))
	if err != nil {
		return fmt.Errorf("derive session key: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.EnsureFreshAccess(ctx, serverAddr, sessionKey); err != nil {
		return fmt.Errorf("refresh session: %w", err)
	}
	// После возможного обновления берём актуальный токен
	accessToken = s.AccessToken

	ctxSync, cancelSync := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelSync()
	_ = syncclient.FullSync(ctxSync, localStore, s.UserID, s.AccessToken, serverAddr)
	return nil
}

func runSync(ctx context.Context, localStore *store.Store, userID, accessToken, serverAddr string) {
	g, gCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return syncclient.FullSync(gCtx, localStore, userID, accessToken, serverAddr)
	})
	if err := g.Wait(); err != nil {
		fmt.Fprintf(os.Stderr, "Background sync failed: %v\n", err)
	}
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

		regKey, err := crypto.DeriveKey(password, []byte("gophkeepston-reg-salt"))
		if err != nil {
			return fmt.Errorf("derive reg key: %w", err)
		}
		encSecret, err := crypto.Encrypt(secret, regKey)
		if err != nil {
			return fmt.Errorf("encrypt secret: %w", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		client, err := authclient.NewClient(serverAddr)
		if err != nil {
			return err
		}
		defer client.Close()

		uid, err := client.Register(ctx, login, encSecret)
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

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		client, err := authclient.NewClient(serverAddr)
		if err != nil {
			return err
		}
		defer client.Close()

		resp, err := client.Login(ctx, login)
		if err != nil {
			return fmt.Errorf("login: %w", err)
		}

		regKey, err := crypto.DeriveKey(password, []byte("gophkeepston-reg-salt"))
		if err != nil {
			return fmt.Errorf("derive reg key: %w", err)
		}
		secret, err := crypto.Decrypt(resp.EncryptedSecret, regKey)
		if err != nil {
			return fmt.Errorf("invalid master password: %w", err)
		}

		masterKey, err := crypto.DeriveKey(append(secret, password...), resp.Salt)
		if err != nil {
			return fmt.Errorf("derive master key: %w", err)
		}
		userID = login

		localStore, err = store.NewStore("gophkeepston.db")
		if err != nil {
			return fmt.Errorf("open store: %w", err)
		}

		sessionKey, err := crypto.DeriveKey(append(secret, password...), resp.Salt)
		if err != nil {
			return fmt.Errorf("derive session key: %w", err)
		}
		if err := session.Save(sessionKey, userID, masterKey, resp.AccessToken, resp.RefreshToken); err != nil {
			return fmt.Errorf("save session: %w", err)
		}

		ctxSync, cancelSync := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancelSync()
		if err := syncclient.FullSync(ctxSync, localStore, userID, resp.AccessToken, serverAddr); err != nil {
			// логируем, но не прерываем
			fmt.Printf("Warning: initial sync failed: %v\n", err)
		}

		fmt.Println("Logged in successfully. Session saved.")
		return nil
	},
}

var addCmd = &cobra.Command{
	Use:     "add [type]",
	Short:   "Add a new entry",
	Args:    cobra.ExactArgs(1),
	PreRunE: requireSession,
	RunE: func(cmd *cobra.Command, args []string) error {
		typ := args[0]
		var entry domainentry.Entry
		var err error

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
			entry = domainentry.PasswordEntry{
				ID:       "",
				Type:     domainentry.TypePassword,
				Site:     site,
				Login:    login,
				Password: string(pass),
				Meta:     meta,
			}
		case "text":
			fmt.Print("Title: ")
			var title string
			fmt.Scanln(&title)
			fmt.Print("Content: ")
			var content string
			fmt.Scanln(&content)
			fmt.Print("Meta (optional): ")
			var meta string
			fmt.Scanln(&meta)
			entry = domainentry.TextEntry{
				ID:      "",
				Type:    domainentry.TypeText,
				Title:   title,
				Content: content,
				Meta:    meta,
			}
		case "card":
			fmt.Print("Card number: ")
			var number string
			fmt.Scanln(&number)
			fmt.Print("Expiry (MM/YY): ")
			var expiry string
			fmt.Scanln(&expiry)
			fmt.Print("CVV: ")
			var cvv string
			fmt.Scanln(&cvv)
			fmt.Print("Holder name: ")
			var holder string
			fmt.Scanln(&holder)
			fmt.Print("Meta (optional): ")
			var meta string
			fmt.Scanln(&meta)
			entry = domainentry.CardEntry{
				ID:     "",
				Type:   domainentry.TypeCard,
				Number: number,
				Expiry: expiry,
				CVV:    cvv,
				Holder: holder,
				Meta:   meta,
			}
		case "binary":
			fmt.Print("File path: ")
			var path string
			fmt.Scanln(&path)
			data, e := os.ReadFile(path)
			if e != nil {
				return fmt.Errorf("read file: %w", e)
			}
			fileName := filepath.Base(path)
			fmt.Print("Meta (optional): ")
			var meta string
			fmt.Scanln(&meta)
			entry = domainentry.BinaryEntry{
				ID:       "",
				Type:     domainentry.TypeBinary,
				FileName: fileName,
				Data:     data,
				Meta:     meta,
			}
		default:
			return fmt.Errorf("unsupported type: %s", typ)
		}

		plain, err := domainentry.Marshal(entry)
		if err != nil {
			return fmt.Errorf("marshal: %w", err)
		}
		ciphertext, err := crypto.Encrypt(plain, masterKey)
		if err != nil {
			return fmt.Errorf("encrypt: %w", err)
		}

		entryID := fmt.Sprintf("%s-%d", typ, time.Now().UnixNano())
		if err := localStore.Put(userID, entryID, ciphertext); err != nil {
			return fmt.Errorf("store: %w", err)
		}
		ver := time.Now().UnixNano()
		_ = localStore.PutVersion(userID, entryID, ver)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		runSync(ctx, localStore, userID, accessToken, serverAddr)

		fmt.Println("Entry added and synced:", entryID)
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
		e, err := domainentry.Unmarshal(plain)
		if err != nil {
			return fmt.Errorf("unmarshal entry: %w", err)
		}

		switch v := e.(type) {
		case domainentry.PasswordEntry:
			fmt.Printf("Site: %s\nLogin: %s\nPassword: %s\nMeta: %s\n", v.Site, v.Login, v.Password, v.Meta)
		case domainentry.TextEntry:
			fmt.Printf("Title: %s\nContent: %s\nMeta: %s\n", v.Title, v.Content, v.Meta)
		case domainentry.CardEntry:
			fmt.Printf("Number: %s\nExpiry: %s\nCVV: %s\nHolder: %s\nMeta: %s\n", v.Number, v.Expiry, v.CVV, v.Holder, v.Meta)
		case domainentry.BinaryEntry:
			outPath := v.FileName + ".extracted"
			if err := os.WriteFile(outPath, v.Data, 0644); err != nil {
				return fmt.Errorf("write file: %w", err)
			}
			fmt.Printf("Binary saved to %s\nMeta: %s\n", outPath, v.Meta)
		default:
			return fmt.Errorf("unknown entry type: %T", v)
		}
		return nil
	},
}

var listCmd = &cobra.Command{
	Use:     "list",
	Short:   "List all entry IDs",
	PreRunE: requireSession,
	RunE: func(cmd *cobra.Command, args []string) error {
		ids, err := localStore.List(userID)
		if err != nil {
			return err
		}
		for _, id := range ids {
			fmt.Println(id)
		}
		return nil
	},
}

var deleteCmd = &cobra.Command{
	Use:     "delete [entryID]",
	Short:   "Delete an entry",
	Args:    cobra.ExactArgs(1),
	PreRunE: requireSession,
	RunE: func(cmd *cobra.Command, args []string) error {
		entryID := args[0]
		_ = localStore.PutVersion(userID, entryID, -1)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// [Пункт 03] Рефакторинг на errgroup
		runSync(ctx, localStore, userID, accessToken, serverAddr)

		fmt.Printf("Marked %s for deletion\n", entryID)
		return nil
	},
}

func main() {
	rootCmd.AddCommand(registerCmd, loginCmd, addCmd, getCmd, listCmd, deleteCmd)
	fmt.Printf("GophKeeper | Build: %s | Date: %s\n", buildVersion, buildDate)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
