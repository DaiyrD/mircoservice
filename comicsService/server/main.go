package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	_ "github.com/lib/pq"
	"google.golang.org/grpc"

	pb "comicService/comicserver/test" // Update the import path
)

const (
	port     = ":50051"
	host     = "localhost"
	portDB   = 5432
	user     = "postgres"
	password = "76205527"
	dbname   = "bookstore"
)

type server struct {
	pb.UnimplementedComicsServiceServer
	db *sql.DB
}

func (s *server) CreateComic(ctx context.Context, req *pb.CreateComicRequest) (*pb.Comic, error) {
	comic := req.GetComic()

	// Generate a unique ID for the comic
	var id int64

	// Prepare the SQL statement
	sqlStatement := `
		INSERT INTO comics ( title, author, year, language, price, quantity, publisher)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`

	// Execute the SQL statement
	err := s.db.QueryRowContext(
		ctx,
		sqlStatement,

		comic.GetTitle(),
		comic.GetAuthor(),
		comic.GetYear(),
		comic.GetLanguage(),
		comic.GetPrice(),
		comic.GetQuantity(),
		comic.GetPublisher(),
	).Scan(&id)
	if err != nil {
		log.Printf("Failed to create comic: %v", err)
		return nil, err
	}

	// Set the generated ID and return the created comic
	comic.Id = id
	return comic, nil
}

func (s *server) ReadComic(ctx context.Context, req *pb.ReadComicRequest) (*pb.Comic, error) {
	// Get the comic ID from the request
	id := req.GetId()

	// Prepare the SQL statement
	sqlStatement := `
		SELECT id, title, author, year, language, price, quantity, publisher
		FROM comics
		WHERE id = $1
	`

	// Execute the SQL statement
	row := s.db.QueryRowContext(ctx, sqlStatement, id)

	// Create a Comic object to store the retrieved data
	comic := &pb.Comic{}

	// Scan the row into the Comic object
	err := row.Scan(
		&comic.Id,
		&comic.Title,
		&comic.Author,
		&comic.Year,
		&comic.Language,
		&comic.Price,
		&comic.Quantity,
		&comic.Publisher,
	)
	if err != nil {
		log.Printf("Failed to read comic: %v", err)
		return nil, err
	}

	return comic, nil
}

func (s *server) UpdateComic(ctx context.Context, req *pb.UpdateComicRequest) (*pb.Comic, error) {
	// Get the comic ID and updated data from the request
	id := req.GetId()
	updatedComic := req.GetComic()

	// Prepare the SQL statement
	sqlStatement := `
		UPDATE comics
		SET title = $1, author = $2, year = $3, language = $4, price = $5, quantity = $6, publisher = $7
		WHERE id = $8
		RETURNING id
	`

	// Execute the SQL statement
	err := s.db.QueryRowContext(
		ctx,
		sqlStatement,
		updatedComic.GetTitle(),
		updatedComic.GetAuthor(),
		updatedComic.GetYear(),
		updatedComic.GetLanguage(),
		updatedComic.GetPrice(),
		updatedComic.GetQuantity(),
		updatedComic.GetPublisher(),
		id,
	).Scan(&id)
	if err != nil {
		log.Printf("Failed to update comic: %v", err)
		return nil, err
	}

	// Set the updated ID and return the updated comic
	updatedComic.Id = id
	return updatedComic, nil
}

func (s *server) DeleteComic(ctx context.Context, req *pb.DeleteComicRequest) (*pb.DeleteComicResponse, error) {
	// Get the comic ID from the request
	id := req.GetId()

	// Prepare the SQL statement
	sqlStatement := `
		DELETE FROM comics
		WHERE id = $1
	`

	// Execute the SQL statement
	_, err := s.db.ExecContext(ctx, sqlStatement, id)
	if err != nil {
		log.Printf("Failed to delete comic: %v", err)
		return nil, err
	}

	// Return a success response
	response := &pb.DeleteComicResponse{
		Success: true,
	}
	return response, nil
}

func main() {
	// Create a database connection
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		host, portDB, user, password, dbname)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Ping the database to verify the connection
	err = db.Ping()
	if err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	// Create the gRPC server
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterComicsServiceServer(s, &server{db: db})

	// Start serving gRPC requests
	log.Printf("gRPC server listening on %s", port)
	go func() {
		err = s.Serve(lis)
		if err != nil {
			log.Fatalf("Failed to serve gRPC: %v", err)
		}
	}()

	// Start serving gRPC-Gateway requests
	mux := runtime.NewServeMux()
	opts := []grpc.DialOption{grpc.WithInsecure()}
	err = pb.RegisterComicsServiceHandlerFromEndpoint(context.Background(), mux, fmt.Sprintf("localhost%s", port), opts)
	if err != nil {
		log.Fatalf("Failed to register gRPC-Gateway: %v", err)
	}

	log.Println("gRPC-Gateway server listening on :8080")
	err = http.ListenAndServe(":8080", mux)
	if err != nil {
		log.Fatalf("Failed to serve gRPC-Gateway: %v", err)
	}
}
