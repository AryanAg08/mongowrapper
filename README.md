# MongoWrapper

A lightweight MongoDB wrapper for Go that provides a fluent interface for database operations.

## Installation

```bash
go get github.com/yourorg/logSense-api/mongowrapper
```

## Features

- Fluent query interface
- Simplified CRUD operations
- Aggregation pipeline support
- Connection pooling
- Index management
- Bulk operations

## Usage

### Connecting to MongoDB

```go
package main

import (
    "context"
    "log"
    "time"
    
    "github.com/yourorg/logSense-api/mongowrapper"
    "go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
    ctx := context.Background()
    
    // Basic connection
    client, err := mongowrap.Connect(ctx, "mongodb://localhost:27017")
    if err != nil {
        log.Fatal(err)
    }
    defer client.Disconnect(ctx)
    
    // Connection with options
    opts := options.Client().
        SetMaxPoolSize(100).
        SetMinPoolSize(10).
        SetMaxConnIdleTime(30 * time.Second)
    
    client, err = mongowrap.Connect(ctx, "mongodb://localhost:27017", opts)
    if err != nil {
        log.Fatal(err)
    }
}
```

### Database and Collection Access

```go
// Get database
db := client.DB("mydb")

// Get collection
collection := db.C("users")
```

### Query Operations

#### Find All Documents

```go
var users []User
err := db.C("users").Find(ctx, bson.M{"active": true}).All(&users)
```

#### Find with Filter and Sort

```go
var users []User
err := db.C("users").
    Find(ctx, bson.M{"age": bson.M{"$gte": 18}}).
    Filter(bson.M{"name": 1, "email": 1}).
    Sort(bson.D{{"name", 1}}).
    Skip(10).
    Limit(20).
    All(&users)
```

#### Find One Document

```go
var user User
err := db.C("users").
    Find(ctx, bson.M{"_id": userID}).
    One(&user)
```

#### Count Documents

```go
var count int64
err := db.C("users").
    Find(ctx, bson.M{"active": true}).
    Count(&count)
```

#### Distinct Values

```go
values, err := db.C("users").
    Find(ctx, bson.M{}).
    Distinct("country")
```

### Insert Operations

#### Insert Single Document

```go
user := User{Name: "John", Email: "john@example.com"}
err := db.C("users").Insert(ctx, user)
```

#### Insert Multiple Documents

```go
users := []interface{}{
    User{Name: "John"},
    User{Name: "Jane"},
}
err := db.C("users").InsertArray(ctx, users)
```

### Update Operations

#### Update Single Document

```go
err := db.C("users").Update(
    ctx,
    bson.M{"_id": userID},
    bson.M{"$set": bson.M{"name": "John Doe"}},
)
```

#### Update with Upsert

```go
result, err := db.C("users").Upsert(
    ctx,
    bson.M{"email": "john@example.com"},
    bson.M{"$set": bson.M{"name": "John", "active": true}},
)
```

#### Update All Matching Documents

```go
result, err := db.C("users").UpdateAll(
    ctx,
    bson.M{"active": false},
    bson.M{"$set": bson.M{"status": "inactive"}},
)
```

#### Find and Update

```go
var updatedUser User
err := db.C("users").FindOneAndUpdate(
    ctx,
    bson.M{"_id": userID},
    bson.M{"$set": bson.M{"name": "John"}},
    &updatedUser,
    options.FindOneAndUpdate().SetReturnDocument(options.After),
)
```

### Delete Operations

#### Delete Single Document

```go
err := db.C("users").Remove(ctx, bson.M{"_id": userID})
```

#### Delete Multiple Documents

```go
result, err := db.C("users").RemoveAll(ctx, bson.M{"active": false})
fmt.Println("Deleted:", result.DeletedCount)
```

### Aggregation Pipeline

```go
pipeline := []bson.M{
    {"$match": bson.M{"active": true}},
    {"$group": bson.M{
        "_id": "$country",
        "count": bson.M{"$sum": 1},
    }},
    {"$sort": bson.M{"count": -1}},
}

var results []bson.M
err := db.C("users").Pipe(ctx, pipeline).All(&results)
```

#### Aggregation with Options

```go
var results []bson.M
err := db.C("users").
    Pipe(ctx, pipeline).
    AllowDiskUse().
    BatchSize(100).
    All(&results)
```

### Index Management

```go
import "go.mongodb.org/mongo-driver/mongo"

indexes := []mongo.IndexModel{
    {
        Keys: bson.D{{"email", 1}},
        Options: options.Index().SetUnique(true),
    },
    {
        Keys: bson.D{{"name", 1}, {"age", -1}},
    },
}

indexNames, err := db.C("users").CreateIndex(ctx, indexes)
```

### Bulk Operations

```go
import "go.mongodb.org/mongo-driver/mongo"

operations := []mongo.WriteModel{
    mongo.NewInsertOneModel().SetDocument(bson.M{"name": "John"}),
    mongo.NewUpdateOneModel().
        SetFilter(bson.M{"name": "Jane"}).
        SetUpdate(bson.M{"$set": bson.M{"age": 30}}),
    mongo.NewDeleteOneModel().SetFilter(bson.M{"name": "Bob"}),
}

result, err := db.C("users").BulkWrite(ctx, operations)
```

## Advanced Features

### Estimated Document Count

For large collections, use estimated count for better performance:

```go
var count int64
err := db.C("users").EstimateCount(ctx, &count)
```

### Using Hints

```go
var users []User
err := db.C("users").
    Find(ctx, bson.M{"age": bson.M{"$gte": 18}}).
    Hint([]string{"age"}).
    All(&users)
```

## Error Handling

```go
import (
    "errors"
    "go.mongodb.org/mongo-driver/mongo"
)

var user User
err := db.C("users").Find(ctx, bson.M{"_id": userID}).One(&user)
if err != nil {
    if errors.Is(err, mongo.ErrNoDocuments) {
        // Handle not found
    } else {
        // Handle other errors
    }
}
```

## Best Practices

1. **Always use context**: Pass context to control timeouts and cancellations
2. **Connection pooling**: Configure appropriate pool sizes for your workload
3. **Index your queries**: Create indexes for frequently queried fields
4. **Use projection**: Only fetch fields you need with Filter()
5. **Batch operations**: Use bulk operations for multiple writes

## License

MIT
