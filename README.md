# WinningSon-inator Backend

This is the backend service for the Winning Son application, built with Go. It provides RESTful APIs for authentication, user management, admin operations, dashboard data, and journal entries.

## Project Structure

```
go.mod
cmd/
  server/
    main.go           # Entry point for the server
internal/
  db/
    migrate.go        # Database migration logic
  handlers/
    admin.go          # Admin-related handlers
    auth.go           # Authentication handlers
    dashboard.go      # Dashboard data handlers
    journal.go        # Journal entry handlers
    user.go           # User management handlers
  middleware/
    auth.go           # Authentication middleware
  models/
    models.go         # Data models
```

## Getting Started

### Prerequisites
- Go 1.20+
- PostgreSQL (or your chosen database)

### Installation
1. Clone the repository:
   ```sh
   git clone https://github.com/HabeebUllahTabrez/winning-son-backend.git
   cd winning-son-backend
   ```
2. Install dependencies:
   ```sh
   go mod tidy
   ```

### Configuration
- Set up your environment variables for database connection and other secrets as needed.
- Update the configuration in `cmd/server/main.go` or use a `.env` file if supported.

### CORS Configuration
The API uses the `github.com/go-chi/cors` middleware to handle cross-origin requests. By default, it allows requests from common development origins:
- `http://localhost:3000`
- `http://localhost:5173`
- `http://127.0.0.1:3000`
- `http://127.0.0.1:5173`

To customize allowed origins, set the `ALLOWED_ORIGINS` environment variable with comma-separated values:
```sh
export ALLOWED_ORIGINS="http://localhost:3000,https://yourdomain.com"
```

The CORS configuration includes:
- Allowed methods: GET, POST, PUT, DELETE, OPTIONS, PATCH
- Allowed headers: Accept, Authorization, Content-Type, X-Requested-With, Origin
- Exposed headers: Link, Content-Length, Content-Type
- Credentials: true (for cookies/auth headers)
- Max age: 300 seconds

### Database Migration
Run migrations using:
```sh
go build ./cmd/server
```

### Running the Server
Start the backend server:
```sh
go run ./cmd/server
```

## API Documentation

This project uses Swagger for API documentation. Once the server is running, you can access the interactive API documentation at:

**Swagger UI:** `http://localhost:8080/swagger/index.html`

The Swagger documentation provides:
- Complete API endpoint reference
- Request/response schemas
- Interactive API testing
- Authentication details

### Regenerating Swagger Documentation

If you make changes to the API handlers or annotations, regenerate the Swagger documentation:

```sh
# Quick way: Use the helper script
./scripts/regenerate-docs.sh

# Manual way:
go install github.com/swaggo/swag/cmd/swag@latest
~/go/bin/swag init -g cmd/server/main.go -o ./docs
```

### Sharing API Documentation

You can export and share the API documentation in multiple formats:

- **`docs/api-documentation-standalone.html`** - Interactive web UI (open in any browser)
- **`docs/Winning-Son-API.postman_collection.json`** - Import into Postman for testing
- **`docs/swagger.json`** or **`docs/swagger.yaml`** - OpenAPI spec for integration/code generation

See [docs/EXPORT-GUIDE.md](docs/EXPORT-GUIDE.md) for detailed instructions on sharing documentation.

## API Endpoints
- **Authentication** (`/api/auth/*`)
  - `POST /api/auth/signup` - User registration
  - `POST /api/auth/login` - User login
- **User Management** (`/api/me`)
  - `GET /api/me` - Get current user profile
  - `PUT /api/me` - Update user profile
  - `GET /api/me/feature-status` - Get feature status
- **Journal** (`/api/journal`)
  - `POST /api/journal` - Create/update journal entry
  - `GET /api/journal` - List journal entries
  - `DELETE /api/journal` - Delete journal entry
- **Dashboard** (`/api/dashboard`)
  - `GET /api/dashboard` - Get dashboard metrics
  - `GET /api/dashboard/submission-history` - Get submission history
- **Admin** (`/api/admin/*`)
  - `GET /api/admin/overview` - Get admin statistics
- **Migration** (`/api/migrate`)
  - `POST /api/migrate` - Migrate user data
- **Analyzer** (`/api/analyzer/*`)
  - `POST /api/analyzer/mark-used` - Mark analyzer as used
- **Health Check**
  - `GET /health` - Health check endpoint

## Contributing
Pull requests are welcome. For major changes, please open an issue first to discuss what you would like to change.

## License
This project is licensed under the MIT License.
