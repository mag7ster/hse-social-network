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
	"time"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

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
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	tokenString := c.GetHeader("Authorization")
	_, claims, err := Authenticate(tokenString)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	id := GetUserIdByClaims(claims)

	grpcReq := &pb.CreatePostRequest{
		Title:       req.Title,
		Description: req.Description,
		CreatorId:   uint32(id),
		IsPrivate:   req.IsPrivate,
		Tags:        req.Tags,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := postsClient.CreatePost(ctx, grpcReq)
	if err != nil {
		log.Printf("gRPC CreatePost error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create post"})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func main() {
	log.Println("I am api gateway")

	userServiceAddr := flag.String("user-service", "", "Address of the user service")
	postsServiceAddr := flag.String("posts-service", "", "Address of the posts service")
	listenPort := flag.Int("port", 8090, "api gateway port")
	flag.Parse()

	if *userServiceAddr == "" || *postsServiceAddr == "" || listenPort == nil {
		flag.Usage()
		os.Exit(1)
	}

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

	addr := fmt.Sprintf(":%d", *listenPort)
	log.Printf("Starting api-gateway on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to run server: %v", err)
	}
}
