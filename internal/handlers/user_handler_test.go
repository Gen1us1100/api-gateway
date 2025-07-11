package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/dgrijalva/jwt-go" // You're using this, ensure it's in go.mod
	"github.com/gen1us1100/go-gateway/internal/models"
	"github.com/gen1us1100/go-gateway/pkg/config"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq" // For pq.Error
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt" // Assuming models.User uses bcrypt
)

// Helper to create a mock user with a hashed password
func mockUser(id, email, username, plainPassword string) (models.User, []string) {
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(plainPassword), bcrypt.DefaultCost)
	return models.User{
		ID:        id,
		Email:     email,
		UserName:  username,
		Password:  string(hashedPassword), // Stored as string in your model
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, []string{"id", "user_name", "email", "password", "created_at", "updated_at"}
}

func TestUserHandler_Login(t *testing.T) {
	cfg := &config.Config{JWTSecret: "testsecret"} // Use a test secret

	// Mock User Data
	testUser, userCols := mockUser(uuid.NewString(), "test@example.com", "testuser", "password123")

	tests := []struct {
		name                 string
		requestBody          interface{}
		mockDBSetup          func(mock sqlmock.Sqlmock)
		expectedStatusCode   int
		expectedResponseBody func(t *testing.T, body string) // More flexible check
	}{
		{
			name: "Successful Login",
			requestBody: LoginRequest{
				Email:    "test@example.com",
				Password: "password123",
			},
			mockDBSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows(userCols).
					AddRow(testUser.ID, testUser.UserName, testUser.Email, testUser.Password, testUser.CreatedAt, testUser.UpdatedAt)
				mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM users WHERE email = $1")).
					WithArgs("test@example.com").
					WillReturnRows(rows)
			},
			expectedStatusCode: http.StatusOK,
			expectedResponseBody: func(t *testing.T, body string) {
				var resp LoginResponse
				err := json.Unmarshal([]byte(body), &resp)
				require.NoError(t, err)
				assert.NotEmpty(t, resp.Token)

				// Optionally, decode and check token claims
				token, err := jwt.Parse(resp.Token, func(token *jwt.Token) (interface{}, error) {
					return []byte(cfg.JWTSecret), nil
				})
				require.NoError(t, err)
				require.True(t, token.Valid)
				claims, ok := token.Claims.(jwt.MapClaims)
				require.True(t, ok)
				assert.Equal(t, testUser.ID, claims["user_id"])
			},
		},
		{
			name: "Invalid JSON Body",
			requestBody: func() interface{} { // Using a func to return malformed string
				return `{"email": "test@example.com", "password":`
			}(),
			mockDBSetup:        func(mock sqlmock.Sqlmock) {}, // No DB interaction expected
			expectedStatusCode: http.StatusBadRequest,
			expectedResponseBody: func(t *testing.T, body string) {
				assert.Contains(t, body, "Invalid request body")
			},
		},
		{
			name: "User Not Found",
			requestBody: LoginRequest{
				Email:    "nonexistent@example.com",
				Password: "password123",
			},
			mockDBSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM users WHERE email = $1")).
					WithArgs("nonexistent@example.com").
					WillReturnError(sql.ErrNoRows) // Simulate user not found
			},
			expectedStatusCode: http.StatusUnauthorized,
			expectedResponseBody: func(t *testing.T, body string) {
				assert.Contains(t, body, "Invalid email or password")
			},
		},
		{
			name: "Incorrect Password",
			requestBody: LoginRequest{
				Email:    "test@example.com",
				Password: "wrongpassword",
			},
			mockDBSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows(userCols).
					AddRow(testUser.ID, testUser.UserName, testUser.Email, testUser.Password, testUser.CreatedAt, testUser.UpdatedAt)
				mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM users WHERE email = $1")).
					WithArgs("test@example.com").
					WillReturnRows(rows)
				// The user.CheckPassword(req.Password) will fail
			},
			expectedStatusCode: http.StatusUnauthorized,
			expectedResponseBody: func(t *testing.T, body string) {
				assert.Contains(t, body, "Invalid email or password")
			},
		},
		{
			name: "Database Error on Get User",
			requestBody: LoginRequest{
				Email:    "test@example.com",
				Password: "password123",
			},
			mockDBSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM users WHERE email = $1")).
					WithArgs("test@example.com").
					WillReturnError(errors.New("simulated db error"))
			},
			expectedStatusCode: http.StatusUnauthorized, // Your code returns this for any Get error
			expectedResponseBody: func(t *testing.T, body string) {
				assert.Contains(t, body, "Invalid email or password")
			},
		},
		{
			name: "Token Signing Error",
			requestBody: LoginRequest{
				Email:    "test@example.com",
				Password: "password123",
			},
			mockDBSetup: func(mock sqlmock.Sqlmock) {
				// This setup assumes JWTSecret is empty or invalid, which is hard to mock
				// directly here as it's from cfg. We'll test the path by using an empty secret for this test case.
				rows := sqlmock.NewRows(userCols).
					AddRow(testUser.ID, testUser.UserName, testUser.Email, testUser.Password, testUser.CreatedAt, testUser.UpdatedAt)
				mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM users WHERE email = $1")).
					WithArgs("test@example.com").
					WillReturnRows(rows)
			},
			expectedStatusCode: http.StatusInternalServerError,
			expectedResponseBody: func(t *testing.T, body string) {
				assert.Contains(t, body, "Error generating token")
			},
			// This specific test case requires a different cfg for token signing error
			// We will handle this by creating a new handler with an empty secret
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			sqlxDB := sqlx.NewDb(db, "sqlmock")
			tt.mockDBSetup(mock)

			currentCfg := cfg
			if tt.name == "Token Signing Error" { // Special case for token signing error
				currentCfg = &config.Config{JWTSecret: ""} // Empty secret to cause signing error
			}
			h := NewUserHandler(sqlxDB, currentCfg)

			var reqBodyBytes []byte
			if reqBodyStr, ok := tt.requestBody.(string); ok { // Handle malformed JSON string case
				reqBodyBytes = []byte(reqBodyStr)
			} else {
				reqBodyBytes, err = json.Marshal(tt.requestBody)
				require.NoError(t, err)
			}

			req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBuffer(reqBodyBytes))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			h.Login(rr, req)

			assert.Equal(t, tt.expectedStatusCode, rr.Code)
			if tt.expectedResponseBody != nil {
				tt.expectedResponseBody(t, rr.Body.String())
			}

			err = mock.ExpectationsWereMet()
			assert.NoError(t, err, "SQL mock expectations not met")
		})
	}
}

func TestUserHandler_Register(t *testing.T) {
	cfg := &config.Config{} // Not used directly by Register, but handler needs it

	tests := []struct {
		name                 string
		requestBody          RegisterRequest
		mockDBSetup          func(mock sqlmock.Sqlmock)
		expectedStatusCode   int
		expectedResponseBody func(t *testing.T, body string, rawBody []byte)
	}{
		{
			name: "Successful Registration",
			requestBody: RegisterRequest{
				Email:    "newuser@example.com",
				UserName: "newbie",
				Password: "securepassword123",
			},
			mockDBSetup: func(mock sqlmock.Sqlmock) {
				// Use sqlmock.AnyArg() for dynamic values like ID, hashed_password, created_at, updated_at
				mock.ExpectExec(regexp.QuoteMeta("INSERT INTO users (id, user_name, email, password, created_at, updated_at)")).
					WithArgs(sqlmock.AnyArg(), "newbie", "newuser@example.com", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(1, 1)) // 1 insert id (not used), 1 row affected
			},
			expectedStatusCode: http.StatusCreated,
			expectedResponseBody: func(t *testing.T, body string, rawBody []byte) {
				var userResp models.User
				err := json.Unmarshal(rawBody, &userResp)
				require.NoError(t, err)
				assert.NotEmpty(t, userResp.ID)
				assert.Equal(t, "newuser@example.com", userResp.Email)
				assert.Equal(t, "newbie", userResp.UserName)
				assert.Empty(t, userResp.Password, "Password should not be returned in registration response") // Important!
				assert.NotZero(t, userResp.CreatedAt)
				assert.NotZero(t, userResp.UpdatedAt)
			},
		},
		{
			name: "Invalid JSON Body",
			requestBody: func() RegisterRequest { // To make it distinct, send empty struct, but test actual malformed JSON
				return RegisterRequest{}
			}(),
			// We'll actually send a malformed string in the test runner
			mockDBSetup:        func(mock sqlmock.Sqlmock) {},
			expectedStatusCode: http.StatusBadRequest,
			expectedResponseBody: func(t *testing.T, body string, rawBody []byte) {
				assert.Contains(t, body, "Invalid request body")
			},
		},
		{
			name: "Empty Fields",
			requestBody: RegisterRequest{
				Email:    "",
				UserName: "test",
				Password: "pw",
			},
			mockDBSetup:        func(mock sqlmock.Sqlmock) {},
			expectedStatusCode: http.StatusBadRequest,
			expectedResponseBody: func(t *testing.T, body string, rawBody []byte) {
				assert.Contains(t, body, "Email, username, and password cannot be empty")
			},
		},
		{
			name: "Password Hashing Error", // models.User.HashPassword might fail
			requestBody: RegisterRequest{ // This assumes HashPassword can fail (e.g., if password too long for bcrypt)
				Email:    "test@example.com",
				UserName: "testuser",
				Password: strings.Repeat("a", 73), // bcrypt has a 72 byte limit
			},
			mockDBSetup:        func(mock sqlmock.Sqlmock) {},
			expectedStatusCode: http.StatusInternalServerError,
			expectedResponseBody: func(t *testing.T, body string, rawBody []byte) {
				assert.Contains(t, body, "Error hashing password")
			},
		},
		{
			name: "Duplicate Email",
			requestBody: RegisterRequest{
				Email:    "exists@example.com",
				UserName: "newbie",
				Password: "password123",
			},
			mockDBSetup: func(mock sqlmock.Sqlmock) {
				pqErr := &pq.Error{
					Code:    "23505", // Unique violation
					Message: "duplicate key value violates unique constraint \"users_email_key\"",
					// Other fields can be set if your code checks them
				}
				mock.ExpectExec(regexp.QuoteMeta("INSERT INTO users")).
					WithArgs(sqlmock.AnyArg(), "newbie", "exists@example.com", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnError(pqErr)
			},
			expectedStatusCode: http.StatusConflict,
			expectedResponseBody: func(t *testing.T, body string, rawBody []byte) {
				assert.Contains(t, body, "email already exists")
			},
		},
		{
			name: "Duplicate Username",
			requestBody: RegisterRequest{
				Email:    "new@example.com",
				UserName: "existinguser",
				Password: "password123",
			},
			mockDBSetup: func(mock sqlmock.Sqlmock) {
				pqErr := &pq.Error{
					Code:    "23505", // Unique violation
					Message: "duplicate key value violates unique constraint \"users_user_name_key\"",
				}
				mock.ExpectExec(regexp.QuoteMeta("INSERT INTO users")).
					WithArgs(sqlmock.AnyArg(), "existinguser", "new@example.com", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnError(pqErr)
			},
			expectedStatusCode: http.StatusConflict,
			expectedResponseBody: func(t *testing.T, body string, rawBody []byte) {
				assert.Contains(t, body, "username already exists")
			},
		},
		{
			name: "Other Database Error on Insert",
			requestBody: RegisterRequest{
				Email:    "another@example.com",
				UserName: "anotheruser",
				Password: "password123",
			},
			mockDBSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(regexp.QuoteMeta("INSERT INTO users")).
					WithArgs(sqlmock.AnyArg(), "anotheruser", "another@example.com", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnError(errors.New("generic db error")) // Not a pq.Error with code 23505
			},
			expectedStatusCode: http.StatusInternalServerError,
			expectedResponseBody: func(t *testing.T, body string, rawBody []byte) {
				assert.Contains(t, body, "Error creating user")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			sqlxDB := sqlx.NewDb(db, "sqlmock")
			tt.mockDBSetup(mock)

			h := NewUserHandler(sqlxDB, cfg)

			var reqBodyBytes []byte
			if tt.name == "Invalid JSON Body" {
				reqBodyBytes = []byte(`{"email": "test@example.com", "username":`) // Malformed JSON
			} else {
				reqBodyBytes, err = json.Marshal(tt.requestBody)
				require.NoError(t, err)
			}

			req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBuffer(reqBodyBytes))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			h.Register(rr, req)

			assert.Equal(t, tt.expectedStatusCode, rr.Code)
			if tt.expectedResponseBody != nil {
				tt.expectedResponseBody(t, rr.Body.String(), rr.Body.Bytes())
			}

			err = mock.ExpectationsWereMet()
			assert.NoError(t, err, "SQL mock expectations not met")
		})
	}
}
