# Swagger Documentation Guide

This document provides information about the Swagger/OpenAPI documentation for the Winning Son API.

## Accessing the Documentation

Once the server is running, you can access the Swagger UI at:

**URL:** `http://localhost:8080/swagger/index.html`

## Features

The Swagger documentation provides:

1. **Interactive API Explorer** - Test API endpoints directly from your browser
2. **Request/Response Schemas** - See exactly what data formats are expected
3. **Authentication** - Built-in authentication testing with JWT tokens
4. **Examples** - Sample request bodies for all endpoints

## Using the Swagger UI

### 1. Authentication

Most endpoints require authentication. To test authenticated endpoints:

1. First, use the `/api/auth/signup` or `/api/auth/login` endpoint to get a JWT token
2. Click the "Authorize" button at the top of the page
3. Enter your token in the format: `Bearer YOUR_JWT_TOKEN_HERE`
4. Click "Authorize" to save
5. Now you can test all authenticated endpoints

### 2. Testing Endpoints

1. Click on any endpoint to expand it
2. Click "Try it out"
3. Fill in the required parameters or request body
4. Click "Execute"
5. View the response below

## Regenerating Documentation

After making changes to API handlers or annotations, regenerate the docs:

```bash
# Install swag CLI tool (one-time)
go install github.com/swaggo/swag/cmd/swag@latest

# Generate documentation
~/go/bin/swag init -g cmd/server/main.go -o ./docs
```

## Swagger Annotations

The API uses the following Swagger annotations in the code:

### General API Information (in main.go)
```go
// @title Winning Son API
// @version 1.0
// @description API for journaling, goal tracking, and personal development metrics
// @host localhost:8080
// @BasePath /api
// @securityDefinitions.apikey BearerAuth
```

### Endpoint Annotations (in handlers)
```go
// HandlerName godoc
// @Summary Brief description
// @Description Detailed description
// @Tags tag-name
// @Accept json
// @Produce json
// @Security BearerAuth (for protected endpoints)
// @Param paramName paramType dataType required "description"
// @Success 200 {object} ResponseType
// @Failure 400 {string} string "Error message"
// @Router /endpoint [method]
```

## API Overview

### Public Endpoints
- `POST /api/auth/signup` - User registration
- `POST /api/auth/login` - User authentication

### Protected Endpoints (Require Authentication)

**User Management**
- `GET /api/me` - Get current user profile
- `PUT /api/me` - Update user profile
- `GET /api/me/feature-status` - Get feature completion status

**Journal**
- `POST /api/journal` - Create or update journal entry
- `GET /api/journal` - List journal entries (with optional date filters)
- `DELETE /api/journal` - Delete journal entry

**Dashboard**
- `GET /api/dashboard` - Get dashboard metrics and statistics
- `GET /api/dashboard/submission-history` - Get submission history for date range

**Admin** (Requires admin privileges)
- `GET /api/admin/overview` - Get administrative statistics

**Migration**
- `POST /api/migrate` - Bulk import journal entries and profile data

**Analyzer**
- `POST /api/analyzer/mark-used` - Mark analyzer feature as used

### Health Check
- `GET /health` - Server health check (no auth required)

## Data Models

Key data models used in the API:

### UserDTO
User profile information including personal details, goals, and feature flags.

### journalRequest
```json
{
  "topics": "string",
  "alignment_rating": 1-10,
  "contentment_rating": 1-10,
  "local_date": "YYYY-MM-DD"
}
```

### dashboardResponse
Comprehensive dashboard metrics including karma scores, streaks, and trends.

### credentials
```json
{
  "email": "user@example.com",
  "password": "password"
}
```

## Tips

1. **Date Formats**: All dates should be in `YYYY-MM-DD` format
2. **Ratings**: Alignment and contentment ratings must be between 1-10
3. **JWT Expiry**: Tokens expire after the configured period (default: 7 days)
4. **CORS**: The API supports CORS for configured origins

## Troubleshooting

### Swagger UI not loading
- Ensure the server is running on the correct port
- Check that the `/swagger/*` route is properly configured
- Verify the docs package is imported in main.go: `_ "winsonin/docs"`

### 401 Unauthorized errors
- Make sure you've authorized with a valid JWT token
- Check that the token hasn't expired
- Ensure the token format is: `Bearer YOUR_TOKEN`

### Documentation out of sync
- Run the swag init command to regenerate docs after code changes
- Restart the server to load the new documentation
