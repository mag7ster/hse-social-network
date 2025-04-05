package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"

	pb "hse-social-network/proto/posts_comments"
)

type postsServer struct {
	pb.UnimplementedPostsCommentsServiceServer
	db *DBWrapper
}

func (s *postsServer) CreatePost(ctx context.Context, req *pb.CreatePostRequest) (*pb.PostResponse, error) {

	post := &Post{
		Title:       req.Title,
		Description: req.Description,
		CreatorID:   uint(req.UserId),
		IsPrivate:   req.IsPrivate,
		Tags:        req.Tags,
	}

	if err := s.db.CreatePost(post); err != nil {
		log.Printf("Error creating post: %v", err)
		return nil, status.Error(codes.Internal, "Failed to create post")
	}

	return &pb.PostResponse{
		PostId:      uint32(post.ID),
		Title:       post.Title,
		Description: post.Description,
		CreatorId:   uint32(post.CreatorID),
		CreatedAt:   post.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   post.UpdatedAt.Format(time.RFC3339),
		IsPrivate:   post.IsPrivate,
		Tags:        []string(post.Tags),
	}, nil
}

func (s *postsServer) DeletePost(ctx context.Context, req *pb.DeletePostRequest) (*pb.PostResponse, error) {

	post, err := s.db.GetPostByID(uint(req.PostId))
	if err != nil {
		return nil, status.Error(codes.NotFound, "Post not found")
	}

	if uint(req.UserId) != post.CreatorID {
		return nil, status.Error(codes.PermissionDenied, "You do not have permission to delete this post")
	}

	if err := s.db.DeletePost(uint(req.PostId), uint(req.UserId)); err != nil {
		log.Printf("Error deleting post: %v", err)
		return nil, status.Error(codes.Internal, "Failed to delete post")
	}

	return &pb.PostResponse{
		PostId:      uint32(post.ID),
		Title:       post.Title,
		Description: post.Description,
		CreatorId:   uint32(post.CreatorID),
		CreatedAt:   post.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   post.UpdatedAt.Format(time.RFC3339),
		IsPrivate:   post.IsPrivate,
		Tags:        []string(post.Tags),
	}, nil
}

func (s *postsServer) UpdatePost(ctx context.Context, req *pb.UpdatePostRequest) (*pb.PostResponse, error) {

	if req.PostId == 0 {
		return nil, status.Error(codes.InvalidArgument, "Post ID cannot be empty")
	}
	if req.UserId == 0 {
		return nil, status.Error(codes.InvalidArgument, "User ID cannot be empty")
	}

	post := &Post{
		ID:          uint(req.PostId),
		Title:       req.Title,
		Description: req.Description,
		CreatorID:   uint(req.UserId),
		IsPrivate:   req.IsPrivate,
		Tags:        req.Tags,
	}

	if err := s.db.UpdatePost(post, req.UpdateFlags); err != nil {
		log.Printf("Error updating post: %v", err)
		if err == gorm.ErrRecordNotFound {
			return nil, status.Error(codes.NotFound, "Post not found or you do not have permission to update it")
		}
		return nil, status.Error(codes.Internal, "Failed to update post")
	}

	updatedPost, err := s.db.GetPostByID(uint(req.PostId))
	if err != nil {
		log.Printf("Error retrieving updated post: %v", err)
		return nil, status.Error(codes.Internal, "Failed to retrieve updated post")
	}

	return &pb.PostResponse{
		PostId:      uint32(updatedPost.ID),
		Title:       updatedPost.Title,
		Description: updatedPost.Description,
		CreatorId:   uint32(updatedPost.CreatorID),
		CreatedAt:   updatedPost.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   updatedPost.UpdatedAt.Format(time.RFC3339),
		IsPrivate:   updatedPost.IsPrivate,
		Tags:        []string(updatedPost.Tags),
	}, nil
}

func (s *postsServer) GetPost(ctx context.Context, req *pb.GetPostRequest) (*pb.PostResponse, error) {

	post, err := s.db.GetPostByID(uint(req.PostId))
	if err != nil {
		return nil, status.Error(codes.NotFound, "Post not found")
	}

	hasAccess, err := s.db.CheckPostAccess(uint(req.PostId), uint(req.UserId))
	if err != nil {
		return nil, status.Error(codes.Internal, "Failed to check post access")
	}
	if !hasAccess {
		return nil, status.Error(codes.PermissionDenied, "You do not have permission to view this post")
	}

	return &pb.PostResponse{
		PostId:      uint32(post.ID),
		Title:       post.Title,
		Description: post.Description,
		CreatorId:   uint32(post.CreatorID),
		CreatedAt:   post.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   post.UpdatedAt.Format(time.RFC3339),
		IsPrivate:   post.IsPrivate,
		Tags:        []string(post.Tags),
	}, nil
}

func (s *postsServer) ListPosts(ctx context.Context, req *pb.ListPostsRequest) (*pb.ListPostsResponse, error) {
	posts, err := s.db.ListPosts(req.Page, req.UserId, req.TargetUserId)
	if err != nil {
		log.Printf("Error listing posts: %v", err)
		return nil, status.Error(codes.Internal, "Failed to list posts")
	}

	response := &pb.ListPostsResponse{
		Posts: make([]*pb.PostResponse, 0, len(posts)),
	}

	for _, post := range posts {
		response.Posts = append(response.Posts, &pb.PostResponse{
			PostId:      uint32(post.ID),
			Title:       post.Title,
			Description: post.Description,
			CreatorId:   uint32(post.CreatorID),
			CreatedAt:   post.CreatedAt.Format(time.RFC3339),
			UpdatedAt:   post.UpdatedAt.Format(time.RFC3339),
			IsPrivate:   post.IsPrivate,
			Tags:        []string(post.Tags),
		})
	}

	return response, nil
}

func main() {
	log.Println("I am posts service")

	port := flag.Int("port", 8090, "The server port")
	dbURL := flag.String("db", "", "Database connection string")
	flag.Parse()

	if *dbURL == "" {
		log.Fatal("Database URL is required")
	}

	db, err := InitDB(*dbURL)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterPostsCommentsServiceServer(s, &postsServer{db: db})

	log.Printf("Starting posts service on port %d", *port)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
