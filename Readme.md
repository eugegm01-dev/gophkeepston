# GophKeepston

GophKeepston — это защищённый менеджер паролей и приватных данных с терминальным интерфейсом (TUI).
Проект построен на клиент-серверной архитектуре с использованием gRPC, PostgreSQL и BoltDB.
Данные шифруются на клиенте (AES-256-GCM), сервер не имеет доступа к содержимому (zero-knowledge).

## Основные возможности

- Регистрация и вход с мастер-паролем (zero-knowledge proof).
- Хранение и шифрование паролей, текстовых заметок, банковских карт и бинарных файлов.
- Локальное хранилище BoltDB с прозрачным шифрованием.
- Синхронизация данных между устройствами через сервер (Push/Pull с версионированием).
- Refresh-токены для долгоживущих сессий.
- Терминальный интерфейс (TUI) в стиле dungeon с навигацией по записям.
- CLI-клиент для автоматизации.
- Swagger-документация на API.
- Покрытие unit-тестами > 73%.

## Архитектура


Клиент (CLI / TUI) ─── gRPC ─── Сервер (Go) ─── PostgreSQL
│ │
└── BoltDB (локальное хранилище)


## Быстрый старт

1. Клонируйте репозиторий:
   ```bash
   git clone https://github.com/eugegm01-dev/gophkeepston.git
   cd gophkeepston ```

2.   Поднимите PostgreSQL:

```bash
docker compose up -d
make run-migrations
```
3. Запустите сервер:

```bash
make run-server
```
4. В другом терминале запустите TUI:

```bash
В другом терминале запустите TUI:
```

Или используйте CLI:

```bash
make run-client ARGS="register"
```

## Команды Makefile

Команда	                     Описание
make run-server	            Запуск gRPC сервера
make run-tui	               Запуск терминального интерфейса
make run-client ARGS="..."	   Запуск CLI с аргументами
make run-migrations	         Выполнить миграции БД
make gen-proto	               Сгенерировать код из .proto файлов

## Документация

Swagger UI доступен при запуске сервера по адресу http://localhost:8080/swagger/ (если настроен).
Godoc комментарии доступны в исходном коде.

## Тестирование

```bash

go test ./...
```

## Покрытие: 

```bash
go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out | grep total
```

## Безопасность
- Соли для Argon2 в текущей версии фиксированы. В production соли должны быть уникальными для каждого пользователя.
- JWT‑секрет задаётся через переменную окружения `JWT_SECRET`.

## Сертификаты

openssl req -x509 -newkey rsa:4096 -keyout certs/server.key -out certs/server.crt -days 365 -nodes

## Лицензия
Этот проект распространяется под лицензией MIT. Подробности см. в файле [LICENSE](LICENSE).
