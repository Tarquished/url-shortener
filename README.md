# 🔗 URL Shortener API

A REST API for shortening URLs with click tracking, built with Go and PostgreSQL.

**Live API:** `https://url-shortener-production-d0ce.up.railway.app`

---

## Tech Stack

- **Go** — backend language
- **PostgreSQL** — database
- **GORM** — ORM for database operations
- **JWT** — authentication
- **Railway** — cloud deployment

---

## Endpoints

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| `POST` | `/register` | ❌ | Register new account |
| `POST` | `/login` | ❌ | Login + get JWT token |
| `POST` | `/create` | ✅ | Create short URL |
| `GET` | `/urls` | ✅ | Get all your short URLs |
| `PUT` | `/update?id=X` | ✅ | Update original URL |
| `DELETE` | `/delete?id=X` | ✅ | Delete short URL |
| `GET` | `/:code` | ❌ | Redirect to original URL |

---

## Request & Response Examples

### Register
```http
POST /register
Content-Type: application/json

{
    "username": "jason",
    "password": "rahasia123"
}
```

Response:
```json
{"pesan": "Berhasil menambahkan username ke database"}
```

### Login
```http
POST /login
Content-Type: application/json

{
    "username": "jason",
    "password": "rahasia123"
}
```

Response:
```json
{
    "pesan": "Berhasil login!",
    "token": "eyJhbGci..."
}
```

### Create Short URL
```http
POST /create
Authorization: Bearer eyJhbGci...
Content-Type: application/json

{
    "url": "https://www.tokopedia.com/product/very-long-url-here"
}
```

Response:
```json
{"pesan": "Berhasil menambahkan url ke database"}
```

### Get All URLs
```http
GET /urls
Authorization: Bearer eyJhbGci...
```

Response:
```json
[
    {
        "user_id": 1,
        "originalurl": "https://www.tokopedia.com/product/very-long-url-here",
        "shortcode": "abc123",
        "clickcount": 5
    }
]
```

### Redirect
```http
GET /abc123
```
Redirects to the original URL. Click count increments automatically.

---

## How It Works

```
User submits long URL
        ↓
Server generates 6-char random code (e.g. "abc123")
        ↓
Saved to database with user ownership
        ↓
Anyone visits /:code → redirected to original URL
        ↓
Click count increments on every visit
```

---

## Local Development

**Prerequisites:** Go 1.22+, PostgreSQL

```bash
git clone https://github.com/Tarquished/url-shortener.git
cd url-shortener
go mod tidy
go run main.go
```

Server runs at `http://localhost:8080`.

**Environment Variables:**

| Variable | Description |
|----------|-------------|
| `DATABASE_URL` | PostgreSQL connection string |
| `PORT` | Server port (default: 8080) |
| `JWT_SECRET` | Secret key for JWT signing |
