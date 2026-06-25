# Colday 2026

Музыкальный сайт-визитка с админ-панелью. Сервер на Go, фронтенд — нативная SPA (vanilla JS).

## Архитектура

```
colday2026/
├── main.go                    # Точка входа, конфиг, роутинг
├── internal/
│   ├── models/models.go       # Модель Release
│   ├── store/store.go         # Хранилище: чтение/запись JSON-файла
│   ├── auth/auth.go           # Аутентификация: HMAC-SHA256 сессионные куки
│   └── handlers/              # HTTP обработчики
│       ├── handlers.go        #   общие хелперы (writeJSON, readJSON)
│       ├── releases.go        #   CRUD релизов
│       ├── auth_handlers.go   #   login / logout / me
│       ├── upload.go          #   загрузка обложек
│       └── middleware.go      #   RequireAuth
├── static/
│   ├── colday.html            #   публичная страница
│   ├── goatadmin.html         #   админ-панель
│   └── favicon.svg
├── data/
│   └── data.json              #   файл БД (создаётся автоматически)
├── Dockerfile
└── docker-compose.yml
```

Чёткое разделение слоёв:

| Слой      | Пакет      | Ответственность                               |
|-----------|------------|-----------------------------------------------|
| **API**   | `handlers` | HTTP-роутинг, парсинг запросов, JSON-ответы   |
| **Auth**  | `auth`     | Подпись/верификация HMAC-кук, проверка пароля |
| **Store** | `store`    | CRUD, персистентность, потокобезопасность     |
| **Model** | `models`   | Структуры данных                              |

## Хранилище

Вместо базы данных используется `data/data.json` (настраивается через `COLDAY_DATA_FILE`). Все CRUD-операции
потокобезопасны (`sync.RWMutex`), запись атомарная (write-to-tmp + rename).

Формат файла:

```json
{
  "releases": [
    {
      "id": 1,
      "title": "Название",
      "release_type": "album",
      "cover": "/static/uploads/abc.jpg",
      "bandlink": "https://band.link/...",
      "upc": "1234567890",
      "original_date": "2024-03-15",
      "counter_songs": 10,
      "song_duration": "00:35:00",
      "featured": true
    }
  ],
  "next_id": 2
}
```

При старте файл создаётся автоматически, если его нет.

## API

Все эндпоинты возвращают `Content-Type: application/json`.

### Публичные

| Метод  | Путь                 | Описание                              |
|--------|----------------------|---------------------------------------|
| `GET`  | `/api/releases`      | Список всех релизов (featured первый) |
| `GET`  | `/api/releases/{id}` | Один релиз по ID                      |
| `POST` | `/api/login`         | Вход (тело: `{"password":"..."}`)     |
| `POST` | `/api/logout`        | Выход                                 |
| `GET`  | `/api/me`            | Статус аутентификации                 |

### Требуют аутентификации

| Метод    | Путь                 | Описание                                   |
|----------|----------------------|--------------------------------------------|
| `POST`   | `/api/releases`      | Создать релиз                              |
| `PUT`    | `/api/releases/{id}` | Обновить релиз                             |
| `DELETE` | `/api/releases/{id}` | Удалить релиз                              |
| `POST`   | `/api/upload-cover`  | Загрузить обложку (multipart, поле `file`) |

### Статические файлы

- `/` — публичная страница (`colday.html`)
- `/admin` — админ-панель (`goatadmin.html`)
- `/static/*` — прочие статические файлы

## Запуск

### Локально (Go)

```bash
# Установить переменные окружения
export COLDAY_ADMIN_PASSWORD=secret

# Запустить
go run .
```

### Docker

```bash
# Создать .env из шаблона
echo "COLDAY_ADMIN_PASSWORD=secret" > .env

# Запустить
docker compose up --build
```

## Конфигурация

Все настройки через переменные окружения (или `.env` файл):

| Переменная              | По умолчанию         | Описание                              |
|-------------------------|----------------------|---------------------------------------|
| `COLDAY_ADDR`           | `:8080`              | Адрес сервера                         |
| `COLDAY_ADMIN_PASSWORD` | **обязательно**      | Пароль админа                         |
| `COLDAY_DATA_FILE`      | `data/data.json`     | Путь к файлу хранилища                |
| `COLDAY_SESSION_KEY`    | эфемерный (32 байта) | HMAC-ключ для кук (hex, ≥64 символов) |
| `COLDAY_STATIC_DIR`     | `static`             | Директория со статикой                |
| `COLDAY_UPLOADS_DIR`    | `static/uploads`     | Куда сохранять обложки                |
| `COLDAY_COOKIE_SECURE`  | `""`                 | `"1"` — Secure флаг на куке (HTTPS)   |

## Тесты

```bash
go test -v -cover ./internal/handlers/
```

API покрыто 34 тестами (~83% coverage):

- CRUD релизов (включая featured-логику, граничные случаи)
- Аутентификация (вход/выход, неверный пароль, просроченная кука)
- Загрузка обложек (с файлом, без файла, неизвестное расширение)
- Персистентность данных
