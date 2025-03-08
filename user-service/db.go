package main

import (
	"log"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type User struct {
	ID           uint      `gorm:"primaryKey"`
	Username     string    `gorm:"unique;not null"`
	Email        string    `gorm:"unique;not null"`
	PasswordHash string    `gorm:"not null"`
	CreatedAt    time.Time `gorm:"autoCreateTime"`
	Role         string    `gorm:"not null"`
}

type UserProfile struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    uint      `gorm:"not null;unique"`
	FirstName string    `gorm:"size:100"`
	LastName  string    `gorm:"size:100"`
	Bio       string    `gorm:"size:500"`
	AvatarURL string    `gorm:"size:255"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

type UserSession struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    uint      `gorm:"not null"`
	Token     string    `gorm:"not null;unique"`
	LoginAt   time.Time `gorm:"autoCreateTime"`
	ExpiresAt time.Time `gorm:"not null"`
	IPAddress string    `gorm:"size:45"`
}

type DBWrapper struct {
	db *gorm.DB
}

func InitDB(dbURL string) DBWrapper {
	database, err := gorm.Open(postgres.Open(dbURL), &gorm.Config{})
	if err != nil {
		log.Fatalf("Ошибка подключения к БД: %v", err)
	}
	if err := database.AutoMigrate(&User{}, &UserProfile{}, &UserSession{}); err != nil {
		log.Fatalf("Ошибка миграции БД: %v", err)
	}
	return DBWrapper{
		db: database,
	}
}

func (wr *DBWrapper) CreateUser(user *User) error {
	return wr.db.Create(user).Error
}

func (wr *DBWrapper) CreateEmptyProfile(id uint) error {
	userProfile := UserProfile{
		UserID:    id,
		FirstName: "",
		LastName:  "",
		Bio:       "",
		AvatarURL: "",
	}
	return wr.db.Create(&userProfile).Error
}

func (wr *DBWrapper) GetUserByLogin(login string) (*User, error) {
	var user User
	if err := wr.db.Where("username = ?", login).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (wr *DBWrapper) CreateSession(session *UserSession) error {
	return wr.db.Create(session).Error
}

func (wr *DBWrapper) GetUserByUserId(id uint) (*User, error) {
	var user User
	if err := wr.db.Where("id = ?", id).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (wr *DBWrapper) DeleteSessionByUserId(id uint) {
	wr.db.Where("user_id = ?", id).Delete(&UserSession{})
}

func (wr *DBWrapper) GetProfileByUserId(id uint) (*UserProfile, error) {
	var profile UserProfile
	if err := wr.db.Where("user_id = ?", id).First(&profile).Error; err != nil {
		return nil, err
	}
	return &profile, nil
}

func (wr *DBWrapper) UpdateProfile(profile *UserProfile) {
	wr.db.Save(profile)
}
