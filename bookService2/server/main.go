package main

import (
	"context"
	"fmt"
	"log"
	"net"

	"google.golang.org/grpc"

	pb "Booking/bookserver/test" // Update the import path

	// Import the necessary PostgreSQL library packages
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4"

	// Import the necessary RabbitMQ library packages
	"github.com/streadway/amqp"
)

const (
	port          = ":50051"
	rabbitMQURL   = "amqp://guest:guest@localhost:5672/" // Update with your RabbitMQ URL
	rabbitMQQueue = "book_creation_queue"                // Update with the name of the RabbitMQ queue
)

type server struct {
	pb.UnimplementedBookingServiceServer
	db  *pgx.Conn
	rmq *amqp.Connection
}

func (s *server) CreateBook(ctx context.Context, req *pb.CreateBookRequest) (*pb.Book, error) {
	book := req.GetBook()

	// Prepare the SQL statement
	sqlStatement := `
		INSERT INTO books (title, author, year, language, genres, price, quantity)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`

	// Convert the genres slice to array-compatible format
	genresArray := &pgtype.TextArray{}
	if err := genresArray.Set(book.Genres); err != nil {
		log.Printf("Failed to convert genres to array-compatible format: %v", err)
		return nil, err
	}

	// Execute the SQL statement
	var id int64
	err := s.db.QueryRow(
		ctx,
		sqlStatement,
		book.Title,
		book.Author,
		book.Year,
		book.Language,
		genresArray,
		book.Price,
		book.Quantity,
	).Scan(&id)
	if err != nil {
		log.Printf("Failed to create book: %v", err)
		return nil, err
	}

	// Set the generated ID and return the created book
	book.Id = id

	// Publish a message to RabbitMQ indicating the creation of a new book
	err = s.publishToRabbitMQ(book)
	if err != nil {
		log.Printf("Failed to publish to RabbitMQ: %v", err)
	}

	return book, nil
}
func (s *server) publishToRabbitMQ(book *pb.Book) error {
	ch, err := s.rmq.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	idBytes := []byte(fmt.Sprintf("%d", book.Id)) // Convert the book ID to []byte

	err = ch.Publish(
		"",            // exchange
		rabbitMQQueue, // routing key
		false,         // mandatory
		false,         // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        idBytes, // Publish the book ID
		},
	)
	if err != nil {
		return err
	}

	return nil
}

func (s *server) ReadBook(ctx context.Context, req *pb.ReadBookRequest) (*pb.Book, error) {
	// Get the book ID from the request
	bookID := req.GetId()

	// Prepare the SQL statement
	sqlStatement := `
		SELECT id, title, author, year, language, genres, price, quantity
		FROM books
		WHERE id = $1
	`

	// Execute the SQL statement
	row := s.db.QueryRow(ctx, sqlStatement, bookID)

	// Create a Book object to store the retrieved data
	book := &pb.Book{}

	// Scan the row into the Book object
	err := row.Scan(
		&book.Id,
		&book.Title,
		&book.Author,
		&book.Year,
		&book.Language,
		&book.Genres,
		&book.Price,
		&book.Quantity,
	)
	if err != nil {
		log.Printf("Failed to read book: %v", err)
		return nil, err
	}

	return book, nil
}

func (s *server) UpdateBook(ctx context.Context, req *pb.UpdateBookRequest) (*pb.Book, error) {
	bookID := req.GetId()
	updatedBook := req.GetBook()

	// Prepare the SQL statement
	sqlStatement := `
		UPDATE books
		SET title = $1, author = $2, year = $3, language = $4, genres = $5, price = $6, quantity = $7
		WHERE id = $8
		RETURNING id
	`

	// Convert the genres slice to array-compatible format
	genresArray := &pgtype.TextArray{}
	if err := genresArray.Set(updatedBook.Genres); err != nil {
		log.Printf("Failed to convert genres to array-compatible format: %v", err)
		return nil, err
	}

	// Execute the SQL statement
	var id int64
	err := s.db.QueryRow(
		ctx,
		sqlStatement,
		updatedBook.Title,
		updatedBook.Author,
		updatedBook.Year,
		updatedBook.Language,
		genresArray,
		updatedBook.Price,
		updatedBook.Quantity,
		bookID,
	).Scan(&id)
	if err != nil {
		log.Printf("Failed to update book: %v", err)
		return nil, err
	}

	// Set the updated ID and return the updated book
	updatedBook.Id = id
	return updatedBook, nil
}

func (s *server) DeleteBook(ctx context.Context, req *pb.DeleteBookRequest) (*pb.DeleteBookResponse, error) {
	// Get the book ID from the request
	bookID := req.GetId()

	// Prepare the SQL statement
	sqlStatement := `
		DELETE FROM books
		WHERE id = $1
	`

	// Execute the SQL statement
	_, err := s.db.Exec(ctx, sqlStatement, bookID)
	if err != nil {
		log.Printf("Failed to delete book: %v", err)
		return nil, err
	}

	// Return a success response
	response := &pb.DeleteBookResponse{
		Success: true,
	}
	return response, nil
}

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

	// Establish a connection to RabbitMQ
	rmq, err := amqp.Dial(rabbitMQURL)
	if err != nil {
		log.Fatalf("failed to connect to RabbitMQ: %v", err)
	}

	s := grpc.NewServer()
	pb.RegisterBookingServiceServer(s, &server{db: db, rmq: rmq})
	log.Printf("Server listening on port %s", port)

	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
