# FITZONE — Go + MongoDB (Arabic / English)

FITZONE is a bilingual gym and exercise website built with Go. Exercise records are loaded from MongoDB. On first startup, the app seeds the MongoDB `fitzone.exercises` collection with the 16 bundled exercise records if the collection is empty. Exercise photos are served from `static/images/`.

## Run the full app with Docker Compose (recommended)

Requirements: Docker Desktop with Docker Compose enabled.

1. Extract the ZIP and open PowerShell in the `fitzone_golang_app` folder.
2. Build and start both the app and MongoDB:
   ```powershell
   docker compose up --build
   ```
3. Open http://localhost:8080
4. English: http://localhost:8080/?lang=en
5. Arabic: http://localhost:8080/?lang=ar

MongoDB data is stored in the named Docker volume `mongodb_data`, so it persists across container restarts. To stop the services, press Ctrl+C, then run `docker compose down`. Do not add `-v` if you want to keep the database volume.

## Run Go directly on Windows

You need Go 1.21+ and a MongoDB server available at `mongodb://localhost:27017`.

```powershell
go version
go mod tidy
go run .
```

Optional environment variables:

```powershell
$env:MONGODB_URI = "mongodb://localhost:27017"
$env:MONGODB_DATABASE = "fitzone"
go run .
```

## MongoDB structure

Database: `fitzone`
Collection: `exercises`

Each document stores the exercise key, Arabic/English names, category, difficulty level, sets/reps, descriptions, and image filename. The app seeds the starter exercises only when the collection is empty; it does not overwrite existing database records.

## Important

This is a learning project. The app currently reads exercises from MongoDB; user accounts, workout logs, and progress tracking are not implemented yet. Exercise advice is general information, not a substitute for a qualified trainer or healthcare professional.

## REST API (added)

Base URL: `http://localhost:8080/api`. JSON responses are returned by all API endpoints. Protected endpoints require `Authorization: Bearer <token>` from register/login.

### Authentication
- `POST /api/auth/register` JSON: `{"name":"Mohamed","email":"mohamed@example.com","password":"strong-password"}`
- `POST /api/auth/login` JSON: `{"email":"mohamed@example.com","password":"strong-password"}`
- `GET /api/me` (token required)

Passwords are stored as PBKDF2-HMAC-SHA256 hashes, never as plain text. JWTs expire after 24 hours. Set a strong `JWT_SECRET` environment variable before deployment; the compose default is only for local development.

### Exercises and images
- `GET /api/exercises` (optional search: `?q=squat`)
- `GET /api/exercises/{key}`
- `POST /api/exercises`, `PUT /api/exercises/{key}`, `PATCH /api/exercises/{key}`, `DELETE /api/exercises/{key}` (token required for writes)
- `POST /api/uploads` (token required; multipart form field `image`; JPG/PNG/WEBP, max 5 MB). The response returns an image path such as `/static/uploads/<generated-name>.jpg`. Store that path in the exercise `image` field.

Exercise JSON fields use bilingual names such as `key`, `nameAR`, `nameEN`, `categoryKey`, `categoryAR`, `categoryEN`, `levelAR`, `levelEN`, `setsAR`, `setsEN`, `descAR`, `descEN`, and `image`.

### Weekly plans
- `GET /api/plans` lists only the signed-in user's plans.
- `POST /api/plans` JSON example:
```json
{"name":"Week 1","weekStart":"2026-10-26","days":[{"day":"Monday","exercises":[{"exerciseKey":"pushups","sets":3,"reps":12,"restSeconds":60,"completed":false}]},{"day":"Wednesday","exercises":[{"exerciseKey":"squats","sets":3,"reps":12,"restSeconds":60,"completed":false}]}]}
```
- `GET /api/plans/{id}`, `PUT /api/plans/{id}`, `PATCH /api/plans/{id}`, `DELETE /api/plans/{id}`. Plan ownership is checked on every operation.

### Weight and progress
- `GET /api/progress` lists the signed-in user's entries in date order.
- `POST /api/progress` JSON example: `{"weightKg":82.4,"date":"2026-10-26T00:00:00Z","note":"Morning weigh-in"}`. Date can be omitted to use the current time. Allowed weight range is 20–500 kg.
- `DELETE /api/progress/{id}` removes one of the signed-in user's entries.

### Run locally
1. Copy `.env.example` to `.env` and set `JWT_SECRET` to a random secret at least 32 characters long.
2. Run `docker compose up --build`.
3. Open `http://localhost:8080`; check the API at `http://localhost:8080/api/health`.

The API is implemented in `api.go`; the existing bilingual exercise website remains available. For production, also add rate limiting, HTTPS, email verification/password reset, and role-based authorization for exercise management.
