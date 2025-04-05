package main

import (
	"testing"

	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *DBWrapper {
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)

	db.Exec("CREATE TYPE IF NOT EXISTS \"_text\" AS TEXT")

	err = db.AutoMigrate(&Post{})
	require.NoError(t, err)

	return &DBWrapper{db: db}
}

func createTestPosts(t *testing.T, dbw *DBWrapper) []Post {
	posts := []Post{
		{
			Title:       "Public Post 1",
			Description: "Description 1",
			CreatorID:   1,
			IsPrivate:   false,
			Tags:        pq.StringArray{"tag1", "tag2"},
		},
		{
			Title:       "Private Post",
			Description: "Description 2",
			CreatorID:   1,
			IsPrivate:   true,
			Tags:        pq.StringArray{"private", "secret"},
		},
		{
			Title:       "User 2 Post",
			Description: "Description 3",
			CreatorID:   2,
			IsPrivate:   false,
			Tags:        pq.StringArray{"user2", "public"},
		},
		{
			Title:       "User 2 Private",
			Description: "Description 4",
			CreatorID:   2,
			IsPrivate:   true,
			Tags:        pq.StringArray{"user2", "private"},
		},
	}

	var createdPosts []Post
	for _, post := range posts {
		newPost := post
		err := dbw.CreatePost(&newPost)
		require.NoError(t, err)
		require.NotZero(t, newPost.ID)
		createdPosts = append(createdPosts, newPost)
	}

	return createdPosts
}

func TestCreatePost(t *testing.T) {
	dbw := setupTestDB(t)

	post := &Post{
		Title:       "Test Post",
		Description: "Test Description",
		CreatorID:   1,
		IsPrivate:   false,
		Tags:        pq.StringArray{"test", "create"},
	}

	err := dbw.CreatePost(post)
	assert.NoError(t, err)
	assert.NotZero(t, post.ID)

	savedPost, err := dbw.GetPostByID(post.ID)
	assert.NoError(t, err)
	assert.Equal(t, post.Title, savedPost.Title)
	assert.Equal(t, post.Description, savedPost.Description)
	assert.Equal(t, post.CreatorID, savedPost.CreatorID)
	assert.Equal(t, post.IsPrivate, savedPost.IsPrivate)
	assert.ElementsMatch(t, []string{"test", "create"}, savedPost.Tags)
	assert.False(t, savedPost.CreatedAt.IsZero())
	assert.False(t, savedPost.UpdatedAt.IsZero())
}

func TestGetPostByID(t *testing.T) {
	dbw := setupTestDB(t)
	posts := createTestPosts(t, dbw)

	t.Run("existing post", func(t *testing.T) {
		post, err := dbw.GetPostByID(posts[0].ID)
		assert.NoError(t, err)
		assert.Equal(t, posts[0].ID, post.ID)
		assert.Equal(t, "Public Post 1", post.Title)
	})

	t.Run("non-existing post", func(t *testing.T) {
		post, err := dbw.GetPostByID(999)
		assert.Error(t, err)
		assert.Nil(t, post)
		assert.Equal(t, gorm.ErrRecordNotFound, err)
	})
}

func TestDeletePost(t *testing.T) {
	dbw := setupTestDB(t)
	posts := createTestPosts(t, dbw)

	t.Run("delete own post", func(t *testing.T) {
		err := dbw.DeletePost(posts[0].ID, 1)
		assert.NoError(t, err)

		post, err := dbw.GetPostByID(posts[0].ID)
		assert.Error(t, err)
		assert.Nil(t, post)
	})

	t.Run("delete another user's post", func(t *testing.T) {
		err := dbw.DeletePost(posts[2].ID, 1)
		assert.Error(t, err)
		assert.Equal(t, gorm.ErrRecordNotFound, err)

		post, err := dbw.GetPostByID(posts[2].ID)
		assert.NoError(t, err)
		assert.NotNil(t, post)
	})

	t.Run("delete non-existing post", func(t *testing.T) {
		err := dbw.DeletePost(999, 1)
		assert.Error(t, err)
		assert.Equal(t, gorm.ErrRecordNotFound, err)
	})
}

func TestUpdatePost(t *testing.T) {
	dbw := setupTestDB(t)
	posts := createTestPosts(t, dbw)

	t.Run("update title", func(t *testing.T) {
		post := &Post{
			ID:        posts[0].ID,
			Title:     "Updated Title",
			CreatorID: 1,
		}
		err := dbw.UpdatePost(post, 1<<0)
		assert.NoError(t, err)

		updated, err := dbw.GetPostByID(posts[0].ID)
		assert.NoError(t, err)
		assert.Equal(t, "Updated Title", updated.Title)
		assert.Equal(t, "Description 1", updated.Description)
	})

	t.Run("update multiple fields", func(t *testing.T) {
		post := &Post{
			ID:          posts[1].ID,
			Title:       "New Title",
			Description: "New Description",
			IsPrivate:   false,
			Tags:        pq.StringArray{"new", "tags"},
			CreatorID:   1,
		}
		err := dbw.UpdatePost(post, 1<<0|1<<1|1<<2|1<<3)
		assert.NoError(t, err)

		updated, err := dbw.GetPostByID(posts[1].ID)
		assert.NoError(t, err)
		assert.Equal(t, "New Title", updated.Title)
		assert.Equal(t, "New Description", updated.Description)
		assert.False(t, updated.IsPrivate)
		assert.ElementsMatch(t, []string{"new", "tags"}, updated.Tags)
	})

	t.Run("update another user's post", func(t *testing.T) {
		post := &Post{
			ID:        posts[2].ID,
			Title:     "Attempt to update",
			CreatorID: 1,
		}
		err := dbw.UpdatePost(post, 1<<0)
		assert.Error(t, err)
		assert.Equal(t, gorm.ErrRecordNotFound, err)

		unchanged, err := dbw.GetPostByID(posts[2].ID)
		assert.NoError(t, err)
		assert.Equal(t, "User 2 Post", unchanged.Title)
	})
}

func TestListPosts(t *testing.T) {
	dbw := setupTestDB(t)
	createTestPosts(t, dbw)

	t.Run("list own posts as user 1", func(t *testing.T) {
		posts, err := dbw.ListPosts(0, 1, 1)
		assert.NoError(t, err)
		assert.Len(t, posts, 2)

		for _, post := range posts {
			assert.Equal(t, uint(1), post.CreatorID)
		}
	})

	t.Run("list user 2 posts as user 1", func(t *testing.T) {
		posts, err := dbw.ListPosts(0, 1, 2)
		assert.NoError(t, err)

		assert.Len(t, posts, 1)
		assert.Equal(t, uint(2), posts[0].CreatorID)
		assert.False(t, posts[0].IsPrivate)
	})

	t.Run("list all posts as user 1", func(t *testing.T) {
		posts, err := dbw.ListPosts(0, 1, 0)
		assert.NoError(t, err)

		assert.Len(t, posts, 3)
	})

	t.Run("list with pagination", func(t *testing.T) {
		db := setupTestDB(t)

		for i := 0; i < 15; i++ {
			post := &Post{
				Title:       "Pagination Post " + string(rune('A'+i)),
				Description: "Description",
				CreatorID:   1,
				IsPrivate:   false,
			}
			err := db.CreatePost(post)
			require.NoError(t, err)
		}

		page1, err := db.ListPosts(0, 1, 1)
		assert.NoError(t, err)
		assert.Len(t, page1, 10)

		page2, err := db.ListPosts(1, 1, 1)
		assert.NoError(t, err)
		assert.Len(t, page2, 5)

		assert.NotEqual(t, page1[0].ID, page2[0].ID)
	})
}

func TestCheckPostAccess(t *testing.T) {
	dbw := setupTestDB(t)
	posts := createTestPosts(t, dbw)

	t.Run("own public post", func(t *testing.T) {
		hasAccess, err := dbw.CheckPostAccess(posts[0].ID, 1)
		assert.NoError(t, err)
		assert.True(t, hasAccess)
	})

	t.Run("own private post", func(t *testing.T) {
		hasAccess, err := dbw.CheckPostAccess(posts[1].ID, 1)
		assert.NoError(t, err)
		assert.True(t, hasAccess)
	})

	t.Run("other user's public post", func(t *testing.T) {
		hasAccess, err := dbw.CheckPostAccess(posts[2].ID, 1)
		assert.NoError(t, err)
		assert.True(t, hasAccess)
	})

	t.Run("other user's private post", func(t *testing.T) {
		hasAccess, err := dbw.CheckPostAccess(posts[3].ID, 1)
		assert.NoError(t, err)
		assert.False(t, hasAccess)
	})

	t.Run("non-existing post", func(t *testing.T) {
		hasAccess, err := dbw.CheckPostAccess(999, 1)
		assert.Error(t, err)
		assert.False(t, hasAccess)
		assert.Equal(t, gorm.ErrRecordNotFound, err)
	})
}
