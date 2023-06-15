package main

import (
	"context"
	"log"
	"net"

	pb "UserService/userserver/test" // Update with the correct path

	"google.golang.org/grpc"

	// Import the necessary PostgreSQL library packages

	"github.com/jackc/pgx/v4"
)

const (
	port = ":50051"
)

type server struct {
	pb.UnimplementedUserServiceServer
	db *pgx.Conn
}

// func generateUserID() int32 {
// 	return int32(uuid.New().ID())
// }

func (s *server) RegisterUser(ctx context.Context, req *pb.RegisterUserRequest) (*pb.RegisterUserResponse, error) {
	user := req.GetUser()
	password := req.GetPassword()

	var id int32
	var version int32

	// Prepare the SQL statement
	stmt := `
		INSERT INTO users (name, email, password_hash, activated, roles)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, version
	`

	// Execute the SQL statement with the context
	err := s.db.QueryRow(ctx, stmt,
		user.GetName(), user.GetEmail(), password.GetPlaintext(), false, user.GetRoles(),
	).Scan(&id, &version)
	if err != nil {
		return nil, err
	}

	// Assign the generated ID and version to the user
	user.Id = id

	// Return the registered user
	registeredUser := &pb.User{
		Id:        user.Id,
		Name:      user.GetName(),
		Email:     user.GetEmail(),
		Password:  password.GetPlaintext(),
		Activated: false,
		Roles:     user.GetRoles(),
	}
	return &pb.RegisterUserResponse{
		User: registeredUser,
	}, nil
}

// func (s *server) AuthenticateUser(ctx context.Context, req *pb.AuthenticateUserRequest) (*pb.AuthenticateUserResponse, error) {
// 	// Implement your user authentication logic here
// 	email := req.GetEmail()
// 	password := req.GetPassword()

// 	// Retrieve the user from the database based on the provided email
// 	// ...

// 	// Compare the provided password with the stored password hash
// 	// ...

// 	if passwordMatches {
// 		// Generate a JWT token for the authenticated user
// 		// ...

// 		// Return the token and the authenticated user
// 		return &pb.AuthenticateUserResponse{
// 			Token: token,
// 			User:  authenticatedUser,
// 		}, nil
// 	}

// 	// Return an error if authentication fails
// 	return nil, errors.New("authentication failed")
// }

func main() {
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	// Establish a connection to the PostgreSQL database
	db, err := pgx.Connect(context.Background(), "postgres://postgres:76205527@localhost:5432/bookstore?sslmode=disable")
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	s := grpc.NewServer()
	pb.RegisterUserServiceServer(s, &server{db: db})
	log.Printf("Server listening on port %s", port)

	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
