package mongowrapper

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// Client wraps the MongoDB client
type Client struct {
	*mongo.Client
}

// Database wraps the MongoDB database
type Database struct {
	*mongo.Database
}

// Collection wraps the MongoDB collection
type Collection struct {
	*mongo.Collection
}

// Query provides a fluent interface for MongoDB queries
type Query struct {
	collection *mongo.Collection
	ctx        context.Context
	filter     interface{}
	projection interface{}
	sortFields interface{}
	skipCount  *int64
	limitCount *int64
	hint       interface{}
}

// Connect creates a new MongoDB client connection
func Connect(ctx context.Context, uri string, opts ...*options.ClientOptions) (*Client, error) {
	clientOpts := options.Client().ApplyURI(uri)
	for _, opt := range opts {
		clientOpts = clientOpts.SetAppName(*opt.AppName)
		if opt.MaxPoolSize != nil {
			clientOpts = clientOpts.SetMaxPoolSize(*opt.MaxPoolSize)
		}
		if opt.MinPoolSize != nil {
			clientOpts = clientOpts.SetMinPoolSize(*opt.MinPoolSize)
		}
		if opt.MaxConnIdleTime != nil {
			clientOpts = clientOpts.SetMaxConnIdleTime(*opt.MaxConnIdleTime)
		}
	}

	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		return nil, err
	}

	// Ping the database to verify connection
	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		return nil, err
	}

	return &Client{Client: client}, nil
}

// DB returns a handle to a database
func (c *Client) DB(name string) *Database {
	return &Database{Database: c.Database(name)}
}

// C returns a handle to a collection
func (db *Database) C(name string) *Collection {
	return &Collection{Collection: db.Collection(name)}
}

// Find initiates a query
func (c *Collection) Find(ctx context.Context, filter interface{}) *Query {
	return &Query{
		collection: c.Collection,
		ctx:        ctx,
		filter:     filter,
	}
}

// Filter sets the projection for the query
func (q *Query) Filter(projection interface{}) *Query {
	q.projection = projection
	return q
}

// Sort sets the sort order for the query
func (q *Query) Sort(sort interface{}) *Query {
	q.sortFields = sort
	return q
}

// Skip sets the number of documents to skip
func (q *Query) Skip(skip int64) *Query {
	q.skipCount = &skip
	return q
}

// Limit sets the maximum number of documents to return
func (q *Query) Limit(limit int64) *Query {
	q.limitCount = &limit
	return q
}

// Hint sets the index hint for the query
func (q *Query) Hint(hint interface{}) *Query {
	q.hint = hint
	return q
}

// All executes the query and decodes all results
func (q *Query) All(result interface{}) error {
	opts := options.Find()
	if q.projection != nil {
		opts.SetProjection(q.projection)
	}
	if q.sortFields != nil {
		opts.SetSort(q.sortFields)
	}
	if q.skipCount != nil {
		opts.SetSkip(*q.skipCount)
	}
	if q.limitCount != nil {
		opts.SetLimit(*q.limitCount)
	}
	if q.hint != nil {
		opts.SetHint(q.hint)
	}

	cursor, err := q.collection.Find(q.ctx, q.filter, opts)
	if err != nil {
		return err
	}
	defer cursor.Close(q.ctx)

	return cursor.All(q.ctx, result)
}

// One executes the query and decodes a single result
func (q *Query) One(result interface{}, opts ...*options.FindOneOptions) error {
	opt := options.FindOne()
	if len(opts) > 0 && opts[0] != nil {
		opt = opts[0]
	}
	if q.projection != nil {
		opt.SetProjection(q.projection)
	}
	if q.sortFields != nil {
		opt.SetSort(q.sortFields)
	}
	if q.skipCount != nil {
		opt.SetSkip(*q.skipCount)
	}
	if q.hint != nil {
		opt.SetHint(q.hint)
	}

	return q.collection.FindOne(q.ctx, q.filter, opt).Decode(result)
}

// Count returns the count of documents matching the query
func (q *Query) Count(count *int64) error {
	opts := options.Count()
	if q.skipCount != nil {
		opts.SetSkip(*q.skipCount)
	}
	if q.limitCount != nil {
		opts.SetLimit(*q.limitCount)
	}

	c, err := q.collection.CountDocuments(q.ctx, q.filter, opts)
	if err != nil {
		return err
	}
	*count = c
	return nil
}

// Distinct returns distinct values for a field
func (q *Query) Distinct(field string) ([]interface{}, error) {
	return q.collection.Distinct(q.ctx, field, q.filter)
}

// Insert inserts a single document
func (c *Collection) Insert(ctx context.Context, document interface{}) error {
	_, err := c.InsertOne(ctx, document)
	return err
}

// InsertArray inserts multiple documents
func (c *Collection) InsertArray(ctx context.Context, documents []interface{}) error {
	_, err := c.InsertMany(ctx, documents)
	return err
}

// Update updates a single document
func (c *Collection) Update(ctx context.Context, filter interface{}, update interface{}) error {
	_, err := c.UpdateOne(ctx, filter, update)
	return err
}

// Upsert updates or inserts a document
func (c *Collection) Upsert(ctx context.Context, filter interface{}, update interface{}) (*mongo.UpdateResult, error) {
	opts := options.Update().SetUpsert(true)
	return c.UpdateOne(ctx, filter, update, opts)
}

// UpdateAll updates all matching documents
func (c *Collection) UpdateAll(ctx context.Context, filter interface{}, update interface{}) (*mongo.UpdateResult, error) {
	return c.UpdateMany(ctx, filter, update)
}

// FindOneAndUpdate finds and updates a single document
func (c *Collection) FindOneAndUpdate(ctx context.Context, filter interface{}, update interface{}, result interface{}, opts ...*options.FindOneAndUpdateOptions) error {
	var opt *options.FindOneAndUpdateOptions
	if len(opts) > 0 {
		opt = opts[0]
	}
	return c.Collection.FindOneAndUpdate(ctx, filter, update, opt).Decode(result)
}

// FindOneAndReplace finds and replaces a single document
func (c *Collection) FindOneAndReplace(ctx context.Context, filter interface{}, replacement interface{}) error {
	return c.Collection.FindOneAndReplace(ctx, filter, replacement).Err()
}

// FindOneAndReplaceWithUpsert finds and replaces a single document with upsert
func (c *Collection) FindOneAndReplaceWithUpsert(ctx context.Context, filter interface{}, replacement interface{}) error {
	opts := options.FindOneAndReplace().SetUpsert(true)
	return c.Collection.FindOneAndReplace(ctx, filter, replacement, opts).Err()
}

// FindOneAndDelete finds and deletes a single document
func (c *Collection) FindOneAndDelete(ctx context.Context, filter interface{}, update interface{}, result interface{}, opts ...*options.FindOneAndDeleteOptions) error {
	var opt *options.FindOneAndDeleteOptions
	if len(opts) > 0 {
		opt = opts[0]
	}
	return c.Collection.FindOneAndDelete(ctx, filter, opt).Decode(result)
}

// Remove deletes a single document
func (c *Collection) Remove(ctx context.Context, filter interface{}) error {
	_, err := c.DeleteOne(ctx, filter)
	return err
}

// RemoveAll deletes all matching documents
func (c *Collection) RemoveAll(ctx context.Context, filter interface{}) (*mongo.DeleteResult, error) {
	return c.DeleteMany(ctx, filter)
}

// EstimateCount returns an estimated count of documents in the collection
func (c *Collection) EstimateCount(ctx context.Context, count *int64) error {
	c64, err := c.EstimatedDocumentCount(ctx)
	if err != nil {
		return err
	}
	*count = c64
	return nil
}

// Pipe represents an aggregation pipeline query
type Pipe struct {
	collection   *mongo.Collection
	ctx          context.Context
	pipeline     interface{}
	allowDiskUse bool
	batchSize    *int32
	maxTime      *time.Duration
}

// Pipe creates a new aggregation pipeline
func (c *Collection) Pipe(ctx context.Context, pipeline interface{}) *Pipe {
	return &Pipe{
		collection: c.Collection,
		ctx:        ctx,
		pipeline:   pipeline,
	}
}

// AllowDiskUse enables writing to temporary files for large aggregations
func (p *Pipe) AllowDiskUse() *Pipe {
	p.allowDiskUse = true
	return p
}

// BatchSize sets the batch size for the aggregation
func (p *Pipe) BatchSize(size int32) *Pipe {
	p.batchSize = &size
	return p
}

// MaxTime sets the maximum execution time for the aggregation
func (p *Pipe) MaxTime(duration time.Duration) *Pipe {
	p.maxTime = &duration
	return p
}

// All executes the aggregation and decodes all results
func (p *Pipe) All(result interface{}) error {
	opts := options.Aggregate()
	if p.allowDiskUse {
		opts.SetAllowDiskUse(true)
	}
	if p.batchSize != nil {
		opts.SetBatchSize(*p.batchSize)
	}
	if p.maxTime != nil {
		opts.SetMaxTime(*p.maxTime)
	}

	cursor, err := p.collection.Aggregate(p.ctx, p.pipeline, opts)
	if err != nil {
		return err
	}
	defer cursor.Close(p.ctx)

	return cursor.All(p.ctx, result)
}

// One executes the aggregation and decodes a single result
func (p *Pipe) One(result interface{}) error {
	opts := options.Aggregate()
	if p.allowDiskUse {
		opts.SetAllowDiskUse(true)
	}
	if p.batchSize != nil {
		opts.SetBatchSize(*p.batchSize)
	}
	if p.maxTime != nil {
		opts.SetMaxTime(*p.maxTime)
	}

	cursor, err := p.collection.Aggregate(p.ctx, p.pipeline, opts)
	if err != nil {
		return err
	}
	defer cursor.Close(p.ctx)

	if cursor.Next(p.ctx) {
		return cursor.Decode(result)
	}

	return mongo.ErrNoDocuments
}

// CreateIndex creates indexes on the collection
func (c *Collection) CreateIndex(ctx context.Context, models []mongo.IndexModel) ([]string, error) {
	return c.Indexes().CreateMany(ctx, models)
}

// BulkWrite executes bulk write operations
func (c *Collection) BulkWrite(ctx context.Context, operations []mongo.WriteModel, opts ...*options.BulkWriteOptions) (*mongo.BulkWriteResult, error) {
	return c.Collection.BulkWrite(ctx, operations, opts...)
}
