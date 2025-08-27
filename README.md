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

## API Endpoints
- `/auth`      - Authentication (login, register)
- `/user`      - User management
- `/admin`     - Admin operations
- `/dashboard` - Dashboard data
- `/journal`   - Journal entries

## Contributing
Pull requests are welcome. For major changes, please open an issue first to discuss what you would like to change.

## License
This project is licensed under the MIT License.
