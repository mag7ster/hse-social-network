package main

import (
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Post struct {
	ID          uint      `gorm:"primaryKey"`
	Title       string    `gorm:"not null"`
	Description string    `gorm:"not null"`
	CreatorID   uint      `gorm:"not null"`
	CreatedAt   time.Time `gorm:"autoCreateTime"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime"`
	IsPrivate   bool      `gorm:"default:false"`
	Tags        []string  `gorm:"type:text[];serializer:json"`
}

type DBWrapper struct {
	db *gorm.DB
}

func InitDB(dbURL string) (*DBWrapper, error) {
	database, err := gorm.Open(postgres.Open(dbURL), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	if err := database.AutoMigrate(&Post{}); err != nil {
		return nil, err
	}

	return &DBWrapper{
		db: database,
	}, nil
}

func (wr *DBWrapper) CreatePost(post *Post) error {
	return wr.db.Create(post).Error
}

func (wr *DBWrapper) GetPostByID(postID uint) (*Post, error) {
	var post Post
	if err := wr.db.First(&post, postID).Error; err != nil {
		return nil, err
	}
	return &post, nil
}

func (wr *DBWrapper) DeletePost(postID uint, userID uint) error {
	result := wr.db.Where("id = ? AND creator_id = ?", postID, userID).Delete(&Post{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (wr *DBWrapper) UpdatePost(post *Post, updateFlags uint32) error {
	var currentPost Post
	if err := wr.db.First(&currentPost, post.ID).Error; err != nil {
		return err
	}

	if currentPost.CreatorID != post.CreatorID {
		return gorm.ErrRecordNotFound
	}

	updates := map[string]interface{}{}
	if updateFlags&(1<<0) != 0 {
		updates["title"] = post.Title
	}
	if updateFlags&(1<<1) != 0 {
		updates["description"] = post.Description
	}
	if updateFlags&(1<<2) != 0 {
		updates["is_private"] = post.IsPrivate
	}
	if updateFlags&(1<<3) != 0 {
		updates["tags"] = post.Tags
	}

	if len(updates) > 0 {
		if err := wr.db.Model(&currentPost).Updates(updates).Error; err != nil {
			return err
		}
	}

	return nil
}

func (wr *DBWrapper) ListPosts(page uint32, userID uint32, targetUserID uint32) ([]Post, error) {
	var posts []Post
	pageSize := 10
	offset := int(page) * pageSize

	query := wr.db.Order("created_at DESC").Limit(pageSize).Offset(offset)

	// Если запрашиваются посты конкретного пользователя
	if targetUserID > 0 {
		// Если текущий пользователь смотрит чужие посты
		if userID != targetUserID {
			query = query.Where("creator_id = ? AND (is_private = ? OR creator_id = ?)", targetUserID, false, userID)
		} else {
			// Если пользователь смотрит свои посты
			query = query.Where("creator_id = ?", userID)
		}
	} else {
		// Для списка всех постов показываем только публичные посты и собственные приватные
		query = query.Where("is_private = ? OR creator_id = ?", false, userID)
	}

	if err := query.Find(&posts).Error; err != nil {
		return nil, err
	}

	return posts, nil
}

// FilterPostsByTag получает список постов с определенным тегом (для будущего использования)
func (wr *DBWrapper) FilterPostsByTag(tag string, page uint32, userID uint32) ([]Post, error) {
	var posts []Post
	pageSize := 10
	offset := int(page-1) * pageSize

	// Используем оператор ? для поиска тега в массиве
	query := wr.db.Where("? = ANY(tags)", tag).
		Where("is_private = ? OR creator_id = ?", false, userID).
		Order("created_at DESC").
		Limit(pageSize).Offset(offset)

	if err := query.Find(&posts).Error; err != nil {
		return nil, err
	}

	return posts, nil
}

// CheckPostAccess проверяет доступ пользователя к посту
func (wr *DBWrapper) CheckPostAccess(postID uint, userID uint) (bool, error) {
	var post Post
	if err := wr.db.Select("id, creator_id, is_private").First(&post, postID).Error; err != nil {
		return false, err
	}

	// Пользователь может получить доступ, если пост публичный или пользователь является создателем
	return !post.IsPrivate || post.CreatorID == userID, nil
}
