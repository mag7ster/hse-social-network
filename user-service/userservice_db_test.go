package main

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) (*DBWrapper, sqlmock.Sqlmock) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)

	db, err := gorm.Open(postgres.New(postgres.Config{Conn: mockDB, PreferSimpleProtocol: true}), &gorm.Config{})
	assert.NoError(t, err)

	return &DBWrapper{db: db}, mock
}

func TestCreateUser(t *testing.T) {
	wr, mock := setupTestDB(t)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(
		`INSERT INTO "users" ("username","email","password_hash","created_at","role") VALUES ($1,$2,$3,$4,$5) RETURNING "id"`)).
		WithArgs("testuser", "test@example.com", "hash", sqlmock.AnyArg(), "user").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectCommit()

	user := &User{Username: "testuser", Email: "test@example.com", PasswordHash: "hash", Role: "user"}
	err := wr.CreateUser(user)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetUserByLogin(t *testing.T) {
	wr, mock := setupTestDB(t)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "users" WHERE username = $1 ORDER BY "users"."id" LIMIT $2`)).
		WithArgs("testuser", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "email", "password_hash", "role"}).
			AddRow(1, "testuser", "test@example.com", "hash", "user"))

	user, err := wr.GetUserByLogin("testuser")
	assert.NoError(t, err)
	assert.Equal(t, "testuser", user.Username)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateSession(t *testing.T) {
	wr, mock := setupTestDB(t)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(
		`INSERT INTO "user_sessions" ("user_id","token","login_at","expires_at","ip_address") VALUES ($1,$2,$3,$4,$5) RETURNING "id"`)).
		WithArgs(1, "session_token", sqlmock.AnyArg(), sqlmock.AnyArg(), "127.0.0.1").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectCommit()

	session := &UserSession{UserID: 1, Token: "session_token", LoginAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour), IPAddress: "127.0.0.1"}
	err := wr.CreateSession(session)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteSessionByUserId(t *testing.T) {
	wr, mock := setupTestDB(t)

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "user_sessions" WHERE user_id = $1`)).
		WithArgs(1).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	wr.DeleteSessionByUserId(1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetProfileByUserId(t *testing.T) {
	wr, mock := setupTestDB(t)

	mock.ExpectQuery(regexp.QuoteMeta(
		`SELECT * FROM "user_profiles" WHERE user_id = $1 ORDER BY "user_profiles"."id" LIMIT $2`)).
		WithArgs(1, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "first_name", "last_name", "bio", "avatar_url"}).
			AddRow(1, 1, "John", "Doe", "Bio", "avatar_url"))

	profile, err := wr.GetProfileByUserId(1)
	assert.NoError(t, err)
	assert.Equal(t, "John", profile.FirstName)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateProfile(t *testing.T) {
	wr, mock := setupTestDB(t)

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(
		`UPDATE "user_profiles" SET "user_id"=$1,"first_name"=$2,"last_name"=$3,"bio"=$4,"avatar_url"=$5,"updated_at"=$6 WHERE "id" = $7`)).
		WithArgs(0, "Jane", "Doe", "Updated Bio", "new_avatar_url", sqlmock.AnyArg(), 1).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	profile := &UserProfile{
		ID:        1,
		FirstName: "Jane",
		LastName:  "Doe",
		Bio:       "Updated Bio",
		AvatarURL: "new_avatar_url",
	}
	// Если в коде UpdateProfile обновляется также user_id, он должен передаваться,
	// иначе можно оставить 0, как здесь.
	wr.UpdateProfile(profile)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateEmptyProfile(t *testing.T) {
	wr, mock := setupTestDB(t)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(
		`INSERT INTO "user_profiles" ("user_id","first_name","last_name","bio","avatar_url","updated_at") VALUES ($1,$2,$3,$4,$5,$6) RETURNING "id"`)).
		WithArgs(1, "", "", "", "", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectCommit()

	err := wr.CreateEmptyProfile(1)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
