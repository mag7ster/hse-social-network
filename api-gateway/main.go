package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	pb "hse-social-network/proto/posts_comments"
)

var postsClient pb.PostsCommentsServiceClient

func initGRPCClient(address string) error {
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	postsClient = pb.NewPostsCommentsServiceClient(conn)
	return nil
}

func handleCreatePost(c *gin.Context) {
	var req struct {
		Title       string   `json:"title"`
		Description string   `json:"description"`
		IsPrivate   bool     `json:"is_private"`
		Tags        []string `json:"tags"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload: " + err.Error()})
		return
	}

	tokenString := c.GetHeader("Authorization")
	_, claims, err := Authenticate(tokenString)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: " + err.Error()})
		return
	}

	id := GetUserIdByClaims(claims)

	grpcReq := &pb.CreatePostRequest{
		Title:       req.Title,
		Description: req.Description,
		UserId:      uint32(id),
		IsPrivate:   req.IsPrivate,
		Tags:        req.Tags,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := postsClient.CreatePost(ctx, grpcReq)
	if err != nil {
		handleGRPCError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func handleDeletePost(c *gin.Context) {
	postID, err := strconv.ParseUint(c.Param("post_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid post ID"})
		return
	}

	tokenString := c.GetHeader("Authorization")
	_, claims, err := Authenticate(tokenString)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: " + err.Error()})
		return
	}

	userID := GetUserIdByClaims(claims)

	grpcReq := &pb.DeletePostRequest{
		PostId: uint32(postID),
		UserId: uint32(userID),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := postsClient.DeletePost(ctx, grpcReq)
	if err != nil {
		handleGRPCError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func handleUpdatePost(c *gin.Context) {
	postID, err := strconv.ParseUint(c.Param("post_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid post ID"})
		return
	}

	var req struct {
		Title       *string  `json:"title"`
		Description *string  `json:"description"`
		IsPrivate   *bool    `json:"is_private"`
		Tags        []string `json:"tags"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload: " + err.Error()})
		return
	}

	tokenString := c.GetHeader("Authorization")
	_, claims, err := Authenticate(tokenString)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: " + err.Error()})
		return
	}

	userID := GetUserIdByClaims(claims)

	grpcReq := &pb.UpdatePostRequest{
		PostId: uint32(postID),
		Tags:   req.Tags,
		UserId: uint32(userID),
	}

	updateFlags := 0

	if req.Title != nil {
		grpcReq.Title = *req.Title
		updateFlags ^= 1 << 0
	}

	if req.Description != nil {
		grpcReq.Description = *req.Description
		updateFlags ^= 1 << 1
	}

	if req.IsPrivate != nil {
		grpcReq.IsPrivate = *req.IsPrivate
		updateFlags ^= 1 << 2
	}

	if req.Tags != nil {
		grpcReq.Tags = req.Tags
		updateFlags ^= 1 << 3
	}

	grpcReq.UpdateFlags = uint32(updateFlags)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := postsClient.UpdatePost(ctx, grpcReq)
	if err != nil {
		handleGRPCError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func handleGetPost(c *gin.Context) {
	postID, err := strconv.ParseUint(c.Param("post_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid post ID"})
		return
	}

	tokenString := c.GetHeader("Authorization")
	_, claims, err := Authenticate(tokenString)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: " + err.Error()})
		return
	}

	userID := GetUserIdByClaims(claims)

	grpcReq := &pb.GetPostRequest{
		PostId: uint32(postID),
		UserId: uint32(userID),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := postsClient.GetPost(ctx, grpcReq)
	if err != nil {
		handleGRPCError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func handleListPosts(c *gin.Context) {
	pageStr := c.DefaultQuery("page", "1")
	userIDStr := c.DefaultQuery("user_id", "0")

	page, err := strconv.ParseUint(pageStr, 10, 32)
	if err != nil || page < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid page parameter"})
		return
	}

	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user_id parameter"})
		return
	}

	tokenString := c.GetHeader("Authorization")
	_, claims, err := Authenticate(tokenString)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: " + err.Error()})
		return
	}

	currentUserID := GetUserIdByClaims(claims)

	grpcReq := &pb.ListPostsRequest{
		Page:         uint32(page),
		UserId:       uint32(currentUserID),
		TargetUserId: uint32(userID),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := postsClient.ListPosts(ctx, grpcReq)
	if err != nil {
		handleGRPCError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func handleGRPCError(c *gin.Context, err error) {
	st, ok := status.FromError(err)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error: " + err.Error()})
		return
	}

	switch st.Code() {
	case 3: // INVALID_ARGUMENT
		c.JSON(http.StatusBadRequest, gin.H{"error": st.Message()})
	case 5: // NOT_FOUND
		c.JSON(http.StatusNotFound, gin.H{"error": st.Message()})
	case 7: // PERMISSION_DENIED
		c.JSON(http.StatusForbidden, gin.H{"error": st.Message()})
	case 16: // UNAUTHENTICATED
		c.JSON(http.StatusUnauthorized, gin.H{"error": st.Message()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": st.Message()})
	}
}

func main() {
	log.Println("I am api gateway")

	userServiceAddr := flag.String("user-service", "", "Address of the user service")
	postsServiceAddr := flag.String("posts-service", "", "Address of the posts service")
	listenPort := flag.Int("port", 8090, "api gateway port")
	publicKeyPath := flag.String("public", "", "Public key")
	flag.Parse()

	if *userServiceAddr == "" || *postsServiceAddr == "" || listenPort == nil {
		flag.Usage()
		os.Exit(1)
	}

	LoadRSAKeys(*publicKeyPath)

	if err := initGRPCClient(*postsServiceAddr); err != nil {
		log.Fatalf("Failed to connect to posts service: %v", err)
	}

	target, err := url.Parse(*userServiceAddr)
	if err != nil {
		log.Fatalf("Invalid user service URL: %v", err)
	}
	proxy := httputil.NewSingleHostReverseProxy(target)

	r := gin.Default()

	// user service
	userRoutes := []string{"/register", "/login", "/whoami", "/profile/update", "/profile"}
	for _, route := range userRoutes {
		r.Any(route, func(c *gin.Context) {
			proxy.ServeHTTP(c.Writer, c.Request)
		})
	}

	// posts service
	r.POST("/posts/create", handleCreatePost)
	r.DELETE("/posts/:post_id", handleDeletePost)
	r.PUT("/posts/:post_id", handleUpdatePost)
	r.GET("/posts/:post_id", handleGetPost)
	r.GET("/posts", handleListPosts)

	// run the server
	addr := fmt.Sprintf(":%d", *listenPort)
	log.Printf("Starting api-gateway on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to run server: %v", err)
	}
}
