package main

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func setupMockDB(t *testing.T) (*DBWrapper, sqlmock.Sqlmock) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)

	dialector := postgres.New(postgres.Config{
		Conn:                 mockDB,
		PreferSimpleProtocol: true,
	})

	db, err := gorm.Open(dialector, &gorm.Config{})
	require.NoError(t, err)

	return &DBWrapper{db: db}, mock
}

func TestCreatePostWithMock(t *testing.T) {
	wrapper, mock := setupMockDB(t)

	post := &Post{
		Title:       "Test Post",
		Description: "Test Description",
		CreatorID:   1,
		IsPrivate:   false,
		Tags:        pq.StringArray{"tag1", "tag2"},
	}

	mock.ExpectBegin()

	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "posts" ("title","description","creator_id","created_at","updated_at","is_private","tags") VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING "id"`)).
		WithArgs(post.Title, post.Description, post.CreatorID, sqlmock.AnyArg(), sqlmock.AnyArg(), post.IsPrivate, post.Tags).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	mock.ExpectCommit()

	err := wrapper.CreatePost(post)

	assert.NoError(t, err)
	assert.Equal(t, uint(1), post.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetPostByIDWithMock(t *testing.T) {
	wrapper, mock := setupMockDB(t)

	createdAt := time.Now()
	updatedAt := time.Now()

	t.Run("existing post", func(t *testing.T) {
		postID := uint(1)

		rows := sqlmock.NewRows([]string{"id", "title", "description", "creator_id", "created_at", "updated_at", "is_private", "tags"}).
			AddRow(postID, "Test Post", "Test Description", 1, createdAt, updatedAt, false, pq.StringArray{"tag1", "tag2"})

		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "posts" WHERE "posts"."id" = $1 ORDER BY "posts"."id" LIMIT $2`)).
			WithArgs(postID, 1).
			WillReturnRows(rows)

		post, err := wrapper.GetPostByID(postID)

		assert.NoError(t, err)
		assert.NotNil(t, post)
		assert.Equal(t, postID, post.ID)
		assert.Equal(t, "Test Post", post.Title)
		assert.Equal(t, "Test Description", post.Description)
		assert.Equal(t, uint(1), post.CreatorID)
		assert.Equal(t, createdAt.Truncate(time.Second), post.CreatedAt.Truncate(time.Second))
		assert.Equal(t, updatedAt.Truncate(time.Second), post.UpdatedAt.Truncate(time.Second))
		assert.False(t, post.IsPrivate)
		assert.ElementsMatch(t, pq.StringArray{"tag1", "tag2"}, post.Tags)

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("non-existing post", func(t *testing.T) {
		postID := uint(999)

		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "posts" WHERE "posts"."id" = $1 ORDER BY "posts"."id" LIMIT $2`)).
			WithArgs(postID, 1).
			WillReturnError(gorm.ErrRecordNotFound)

		post, err := wrapper.GetPostByID(postID)

		assert.Error(t, err)
		assert.Nil(t, post)
		assert.Equal(t, gorm.ErrRecordNotFound, err)

		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestDeletePostWithMock(t *testing.T) {
	wrapper, mock := setupMockDB(t)

	t.Run("delete own post", func(t *testing.T) {
		postID := uint(1)
		userID := uint(1)

		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "posts" WHERE id = $1 AND creator_id = $2`)).
			WithArgs(postID, userID).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		err := wrapper.DeletePost(postID, userID)

		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("delete other user's post", func(t *testing.T) {
		postID := uint(2)
		userID := uint(1)

		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "posts" WHERE id = $1 AND creator_id = $2`)).
			WithArgs(postID, userID).
			WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectCommit()

		err := wrapper.DeletePost(postID, userID)

		assert.Error(t, err)
		assert.Equal(t, gorm.ErrRecordNotFound, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestUpdatePostWithMock(t *testing.T) {
	wrapper, mock := setupMockDB(t)

	t.Run("update title only", func(t *testing.T) {
		post := &Post{
			ID:        1,
			Title:     "Updated Title",
			CreatorID: 1,
		}

		createdAt := time.Now()
		updatedAt := time.Now()

		rows := sqlmock.NewRows([]string{"id", "title", "description", "creator_id", "created_at", "updated_at", "is_private", "tags"}).
			AddRow(post.ID, "Original Title", "Original Description", post.CreatorID, createdAt, updatedAt, false, pq.StringArray{"tag1", "tag2"})

		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "posts" WHERE "posts"."id" = $1 ORDER BY "posts"."id" LIMIT $2`)).
			WithArgs(post.ID, 1).
			WillReturnRows(rows)

		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta(`UPDATE "posts" SET "title"=$1,"updated_at"=$2 WHERE "id" = $3`)).
			WithArgs(post.Title, sqlmock.AnyArg(), post.ID).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		err := wrapper.UpdatePost(post, 1<<0)

		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("update multiple fields", func(t *testing.T) {
		post := &Post{
			ID:          2,
			Title:       "New Title",
			Description: "New Description",
			IsPrivate:   true,
			Tags:        pq.StringArray{"new", "tags"},
			CreatorID:   1,
		}

		createdAt := time.Now()
		updatedAt := time.Now()

		rows := sqlmock.NewRows([]string{"id", "title", "description", "creator_id", "created_at", "updated_at", "is_private", "tags"}).
			AddRow(post.ID, "Original Title", "Original Description", post.CreatorID, createdAt, updatedAt, false, pq.StringArray{"tag1", "tag2"})

		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "posts" WHERE "posts"."id" = $1 ORDER BY "posts"."id" LIMIT $2`)).
			WithArgs(post.ID, 1).
			WillReturnRows(rows)

		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta(`UPDATE "posts" SET "description"=$1,"is_private"=$2,"tags"=$3,"title"=$4,"updated_at"=$5 WHERE "id" = $6`)).
			WithArgs(post.Description, post.IsPrivate, post.Tags, post.Title, sqlmock.AnyArg(), post.ID).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		err := wrapper.UpdatePost(post, 1<<0|1<<1|1<<2|1<<3) // Обновляем все поля

		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("update other user's post", func(t *testing.T) {
		post := &Post{
			ID:        3,
			Title:     "Attempt to Update",
			CreatorID: 1,
		}

		createdAt := time.Now()
		updatedAt := time.Now()

		rows := sqlmock.NewRows([]string{"id", "title", "description", "creator_id", "created_at", "updated_at", "is_private", "tags"}).
			AddRow(post.ID, "Original Title", "Original Description", 2, createdAt, updatedAt, false, pq.StringArray{"tag1", "tag2"})

		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "posts" WHERE "posts"."id" = $1 ORDER BY "posts"."id" LIMIT $2`)).
			WithArgs(post.ID, 1).
			WillReturnRows(rows)

		err := wrapper.UpdatePost(post, 1<<0)

		assert.Error(t, err)
		assert.Equal(t, gorm.ErrRecordNotFound, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestListPostsWithMock(t *testing.T) {
	wrapper, mock := setupMockDB(t)

	createdAt := time.Now()
	updatedAt := time.Now()

	t.Run("list own posts", func(t *testing.T) {
		userID := uint32(1)
		page := uint32(0)

		rows := sqlmock.NewRows([]string{"id", "title", "description", "creator_id", "created_at", "updated_at", "is_private", "tags"}).
			AddRow(1, "Post 1", "Description 1", userID, createdAt, updatedAt, false, pq.StringArray{"tag1"}).
			AddRow(2, "Post 2", "Description 2", userID, createdAt, updatedAt, true, pq.StringArray{"tag2"})

		mock.ExpectQuery(`SELECT \* FROM "posts" WHERE creator_id = \$1 ORDER BY created_at DESC LIMIT \$2`).
			WithArgs(userID, pageSize).
			WillReturnRows(rows)

		posts, err := wrapper.ListPosts(page, userID, userID)

		assert.NoError(t, err)
		assert.Len(t, posts, 2)
		if len(posts) >= 2 {
			assert.Equal(t, uint(1), posts[0].ID)
			assert.Equal(t, uint(2), posts[1].ID)
			assert.Equal(t, "Post 1", posts[0].Title)
			assert.Equal(t, "Post 2", posts[1].Title)
			assert.Equal(t, uint(userID), posts[0].CreatorID)
			assert.Equal(t, uint(userID), posts[1].CreatorID)
		}

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("list other user's posts", func(t *testing.T) {
		userID := uint32(1)
		targetUserID := uint32(2)
		page := uint32(0)

		rows := sqlmock.NewRows([]string{"id", "title", "description", "creator_id", "created_at", "updated_at", "is_private", "tags"}).
			AddRow(3, "Public Post", "Description", targetUserID, createdAt, updatedAt, false, pq.StringArray{"public"})

		mock.ExpectQuery(`SELECT \* FROM "posts" WHERE creator_id = \$1 AND \(is_private = \$2 OR creator_id = \$3\) ORDER BY created_at DESC LIMIT \$4`).
			WithArgs(targetUserID, false, userID, pageSize).
			WillReturnRows(rows)

		posts, err := wrapper.ListPosts(page, userID, targetUserID)

		assert.NoError(t, err)
		assert.Len(t, posts, 1)
		if len(posts) > 0 {
			assert.Equal(t, uint(3), posts[0].ID)
			assert.Equal(t, "Public Post", posts[0].Title)
			assert.Equal(t, uint(targetUserID), posts[0].CreatorID)
			assert.False(t, posts[0].IsPrivate)
		}

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("list all accessible posts", func(t *testing.T) {
		userID := uint32(1)
		page := uint32(0)

		rows := sqlmock.NewRows([]string{"id", "title", "description", "creator_id", "created_at", "updated_at", "is_private", "tags"}).
			AddRow(1, "Own Post", "Description", userID, createdAt, updatedAt, true, pq.StringArray{"private"}).
			AddRow(3, "Other's Public", "Description", 2, createdAt, updatedAt, false, pq.StringArray{"public"}).
			AddRow(4, "Another Public", "Description", 3, createdAt, updatedAt, false, pq.StringArray{"public"})

		mock.ExpectQuery(`SELECT \* FROM "posts" WHERE is_private = \$1 OR creator_id = \$2 ORDER BY created_at DESC LIMIT \$3`).
			WithArgs(false, userID, pageSize).
			WillReturnRows(rows)

		posts, err := wrapper.ListPosts(page, userID, 0)

		assert.NoError(t, err)
		assert.Len(t, posts, 3)
		if len(posts) >= 3 {
			assert.Equal(t, uint(1), posts[0].ID)
			assert.Equal(t, uint(3), posts[1].ID)
			assert.Equal(t, uint(4), posts[2].ID)
		}

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("list with pagination", func(t *testing.T) {
		userID := uint32(1)
		page := uint32(1)

		rows := sqlmock.NewRows([]string{"id", "title", "description", "creator_id", "created_at", "updated_at", "is_private", "tags"}).
			AddRow(11, "Page 2 Post 1", "Description", userID, createdAt, updatedAt, false, pq.StringArray{"page2"}).
			AddRow(12, "Page 2 Post 2", "Description", userID, createdAt, updatedAt, false, pq.StringArray{"page2"})

		mock.ExpectQuery(`SELECT \* FROM "posts" WHERE creator_id = \$1 ORDER BY created_at DESC LIMIT \$2 OFFSET \$3`).
			WithArgs(userID, pageSize, page*pageSize).
			WillReturnRows(rows)

		posts, err := wrapper.ListPosts(page, userID, userID)

		assert.NoError(t, err)
		assert.Len(t, posts, 2)
		if len(posts) >= 2 {
			assert.Equal(t, uint(11), posts[0].ID)
			assert.Equal(t, uint(12), posts[1].ID)
			assert.Equal(t, "Page 2 Post 1", posts[0].Title)
			assert.Equal(t, "Page 2 Post 2", posts[1].Title)
		}

		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestCheckPostAccessWithMock(t *testing.T) {
	wrapper, mock := setupMockDB(t)

	t.Run("own post access", func(t *testing.T) {
		postID := uint(1)
		userID := uint(1)

		rows := sqlmock.NewRows([]string{"id", "creator_id", "is_private"}).
			AddRow(postID, userID, true)

		mock.ExpectQuery(`SELECT id, creator_id, is_private FROM "posts" WHERE "posts"\."id" = \$1 ORDER BY "posts"\."id" LIMIT \$2`).
			WithArgs(postID, 1).
			WillReturnRows(rows)

		hasAccess, err := wrapper.CheckPostAccess(postID, userID)

		assert.NoError(t, err)
		assert.True(t, hasAccess)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("other's public post access", func(t *testing.T) {
		postID := uint(2)
		userID := uint(1)

		rows := sqlmock.NewRows([]string{"id", "creator_id", "is_private"}).
			AddRow(postID, 2, false)

		mock.ExpectQuery(`SELECT id, creator_id, is_private FROM "posts" WHERE "posts"\."id" = \$1 ORDER BY "posts"\."id" LIMIT \$2`).
			WithArgs(postID, 1).
			WillReturnRows(rows)

		hasAccess, err := wrapper.CheckPostAccess(postID, userID)

		assert.NoError(t, err)
		assert.True(t, hasAccess)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("other's private post no access", func(t *testing.T) {
		postID := uint(3)
		userID := uint(1)

		rows := sqlmock.NewRows([]string{"id", "creator_id", "is_private"}).
			AddRow(postID, 2, true)

		mock.ExpectQuery(`SELECT id, creator_id, is_private FROM "posts" WHERE "posts"\."id" = \$1 ORDER BY "posts"\."id" LIMIT \$2`).
			WithArgs(postID, 1).
			WillReturnRows(rows)

		hasAccess, err := wrapper.CheckPostAccess(postID, userID)

		assert.NoError(t, err)
		assert.False(t, hasAccess)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("non-existent post", func(t *testing.T) {
		postID := uint(999)
		userID := uint(1)

		mock.ExpectQuery(`SELECT id, creator_id, is_private FROM "posts" WHERE "posts"\."id" = \$1 ORDER BY "posts"\."id" LIMIT \$2`).
			WithArgs(postID, 1).
			WillReturnError(gorm.ErrRecordNotFound)

		hasAccess, err := wrapper.CheckPostAccess(postID, userID)

		assert.Error(t, err)
		assert.False(t, hasAccess)
		assert.Equal(t, gorm.ErrRecordNotFound, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
