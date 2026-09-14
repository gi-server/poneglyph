package database

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	Client *mongo.Client
	DB     *mongo.Database
)

// ConnectDB establishes a connection to the MongoDB instance.
func ConnectDB() error {
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		uri = "mongodb://localhost:27017"
	}

	dbName := os.Getenv("MONGO_DB")
	if dbName == "" {
		dbName = "poneglyph"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clientOptions := options.Client().ApplyURI(uri).
		SetMaxPoolSize(50).
		SetMinPoolSize(5).
		SetMaxConnIdleTime(5 * time.Minute)

	var err error
	Client, err = mongo.Connect(ctx, clientOptions)
	if err != nil {
		return fmt.Errorf("failed to connect to mongodb: %w", err)
	}

	if err = Client.Ping(ctx, nil); err != nil {
		return fmt.Errorf("failed to ping mongodb: %w", err)
	}

	DB = Client.Database(dbName)
	log.Printf("Successfully connected to MongoDB database: %s", dbName)
	return nil
}

// GetCollection returns a reference to the specified collection.
func GetCollection(name string) *mongo.Collection {
	return DB.Collection(name)
}

// GetNextSequence generates an atomic auto-increment integer ID for a given collection/sequence.
func GetNextSequence(seqName string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	coll := DB.Collection("counters")
	filter := bson.M{"_id": seqName}
	update := bson.M{"$inc": bson.M{"seq": 1}}

	opts := options.FindOneAndUpdate().
		SetUpsert(true).
		SetReturnDocument(options.After)

	var result struct {
		ID  string `bson:"_id"`
		Seq int    `bson:"seq"`
	}

	err := coll.FindOneAndUpdate(ctx, filter, update, opts).Decode(&result)
	if err != nil {
		return 0, fmt.Errorf("failed to generate sequence for %s: %w", seqName, err)
	}

	return result.Seq, nil
}

// InitSchema sets up necessary indexes for all collections.
func InitSchema() error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// 1. Users Indexes
	usersColl := DB.Collection("users")
	_, err := usersColl.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "username", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create users indexes: %w", err)
	}

	// 2. Jobs Indexes
	jobsColl := DB.Collection("jobs")
	_, err = jobsColl.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "uploaded_at", Value: -1}},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create jobs indexes: %w", err)
	}

	// 3. Audit Logs Indexes
	auditColl := DB.Collection("audit_logs")
	_, err = auditColl.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "created_at", Value: -1}},
		},
		{
			Keys: bson.D{{Key: "actor_id", Value: 1}},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create audit_logs indexes: %w", err)
	}

	log.Println("MongoDB schema and indexes successfully initialized")
	return nil
}
