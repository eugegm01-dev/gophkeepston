package sync

import (
	"context"
	"fmt"
	"time"

	syncpb "github.com/eugegm01-dev/gophkeepston/api/proto/sync"
	"github.com/eugegm01-dev/gophkeepston/internal/client/store"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

var NewClientFunc = NewClient

type Client struct {
	conn  *grpc.ClientConn
	sync  syncpb.SyncClient
	token string
}

func NewClient(addr, token string) (*Client, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}
	return &Client{
		conn:  conn,
		sync:  syncpb.NewSyncClient(conn),
		token: token,
	}, nil
}

func (c *Client) Push(ctx context.Context, entries []*syncpb.Entry) error {
	md := metadata.Pairs("authorization", "Bearer "+c.token)
	ctx = metadata.NewOutgoingContext(ctx, md)

	_, err := c.sync.Push(ctx, &syncpb.PushRequest{Entries: entries})
	return err
}

func (c *Client) Pull(ctx context.Context, sinceVersion int64) ([]*syncpb.Entry, error) {
	md := metadata.Pairs("authorization", "Bearer "+c.token)
	ctx = metadata.NewOutgoingContext(ctx, md)

	resp, err := c.sync.Pull(ctx, &syncpb.PullRequest{SinceVersion: sinceVersion})
	if err != nil {
		return nil, err
	}
	return resp.Entries, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

// Sync выполняет полную синхронизацию: отправляет локальные изменения и получает новые с сервера.
// Возвращает список полученных Entry для обновления локального хранилища.
func FullSync(st *store.Store, userID string, token string, serverAddr string) error {
	cli, err := NewClientFunc(serverAddr, token)
	if err != nil {
		return err
	}
	defer cli.Close()

	// 1. Получаем все локальные ID и их версии
	localIDs, err := st.List(userID)
	if err != nil {
		return err
	}

	// Для простоты будем считать, что локально храним версию внутри зашифрованной записи (расшифровываем, извлекаем версию)
	// Но мы ещё не хранили версию в локальной структуре. Поэтому сейчас сделаем так:
	// При Pull будем получать все записи сервера и сравнивать с локальными по ID.
	// Реализуем по-простому: Pull запрашиваем sinceVersion = 0 всегда, получаем все записи сервера.
	// Затем для каждой записи с сервера: если локально нет – добавляем; если есть и версия сервера > локальной – обновляем.
	// Для Push соберём все локальные записи, которые новее серверных (сравним версии). Пока для простоты возьмём все локальные записи и отправим на сервер с версией, хранящейся в метаданных (добавим версию в зашифрованную структуру).
	// Это временное решение, потом можно оптимизировать.

	// 2. Получаем все записи с сервера
	serverEntries, err := cli.Pull(context.Background(), 0)
	if err != nil {
		return err
	}

	// 3. Обновляем локальное хранилище серверными записями (пропускаем расшифровку, просто сохраняем зашифрованные данные)
	for _, se := range serverEntries {
		// Проверяем, есть ли локально запись с таким ID
		localData, err := st.Get(userID, se.Id)
		if err != nil {
			// Нет локально – просто добавляем
			if err := st.Put(userID, se.Id, se.EncryptedData); err != nil {
				return err
			}
		} else {
			// Есть локально – сравниваем версии (версия хранится внутри зашифрованной записи? Мы не можем расшифровать, чтобы сравнить версию, потому что не знаем мастер-ключ здесь. Значит, синхронизатор должен иметь доступ к мастер-ключу или хранить версию в открытом виде.)
			// Пока оставим так: если запись уже есть, пропускаем (не перезаписываем). Позже сделаем разрешение конфликтов с версиями.
			_ = localData
		}
	}

	// 4. Отправляем на сервер все локальные записи (пока все, без проверки версий)
	for _, id := range localIDs {
		data, err := st.Get(userID, id)
		if err != nil {
			continue
		}
		// Отправляем как новую запись с версией 0 (пока без версионирования)
		entry := &syncpb.Entry{
			Id:            id,
			Type:          "password", // можно извлечь из метаданных
			EncryptedData: data,
			Version:       time.Now().Unix(), // временная версия
			UpdatedAt:     time.Now().Unix(),
		}
		if err := cli.Push(context.Background(), []*syncpb.Entry{entry}); err != nil {
			return err
		}
	}

	return nil
}

func NewClientWithConn(conn *grpc.ClientConn, token string) *Client {
	return &Client{
		conn:  conn,
		sync:  syncpb.NewSyncClient(conn),
		token: token,
	}
}
