# Deploy `autorun-go` to Vercel

## 1. Project setup in Vercel

- Import this repository to Vercel.
- Set **Root Directory** to:
  - `autorun-go`

This project is configured with:
- `api/index.go` as Go Function entry.
- `vercel.json` for runtime and rewrites.

## 2. Environment Variables

Set these in Vercel Project Settings -> Environment Variables:

- `RUN_PHONE`
- `RUN_PASSWORD`
- `ADMIN_TOKEN` (optional but recommended)
- `POSTGRES_URL` (or `DATABASE_URL`) for long-term token storage
- `UPSTASH_REDIS_REST_URL` for Redis cache
- `UPSTASH_REDIS_REST_TOKEN` for Redis auth

Usage modes:
- Normal user: send `phone` + `password` in request body.
- Admin mode: send `adminToken`; backend will use `RUN_PHONE`/`RUN_PASSWORD`.
- If `POSTGRES_URL` + Redis env are configured:
  - login writes token to Postgres (persistent) and Redis (cache)
  - later requests read Redis first, then Postgres fallback

## 3. API endpoint

After deployment, send POST requests to:

- `https://<your-domain>/api`
- `https://<your-domain>/` (also supported by rewrite)

Body examples:

```json
{ "action": "login", "phone": "...", "password": "..." }
```

```json
{ "action": "club_data", "studentId": 123456, "queryDate": "2026-04-01" }
```

```json
{ "action": "club_join", "phone": "...", "password": "...", "activityId": 46994 }
```

```json
{ "action": "session_bootstrap", "studentId": 123456 }
```

## 4. Supported actions

- `login`
- `run`
- `club`
- `club_data`
- `club_join`
- `club_cancel`
- `session_bootstrap`

## 5. Notes

- `map.json` is included for Go Function runtime through `vercel.json` `includeFiles`.
- CORS is enabled as `*` for easier PWA/API integration.
- `tokenSrc` in response marks token source: `redis` / `database` / `login` / `relogin`.
