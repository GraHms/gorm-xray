# GORM X-Ray Plugin

[![Go Reference](https://pkg.go.dev/badge/github.com/grahms/gormxray.svg)](https://pkg.go.dev/github.com/grahms/gormxray)
[![Go Report Card](https://goreportcard.com/badge/github.com/grahms/gormxray)](https://goreportcard.com/report/github.com/grahms/gormxray)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](./LICENSE)

**GORM X-Ray Plugin** seamlessly integrates [AWS X-Ray](https://aws.amazon.com/xray/) with [GORM](https://gorm.io/), enabling you to trace and visualize database operations. It automatically creates annotated X-Ray subsegments for SQL queries, providing deep insights into your data layer’s performance, pinpointing bottlenecks, and making it easier to debug issues.

## Features

- **Automatic Tracing:** Hooks into GORM lifecycle events (Create, Query, Update, Delete, Raw, and Row) without manual instrumentation.
- **Detailed Metadata:** Captures SQL statements, operation types, table names, and affected rows as metadata in each subsegment.
- **Error Recording:** Automatically marks subsegments with errors if queries fail, aiding in fast root-cause analysis.
- **Customizable Formatting:** Redact sensitive information or format queries to highlight performance-critical parts.
- **Lightweight & Performant:** Minimal overhead, ensuring you can safely use this in production environments.

## Installation

```bash
go get github.com/grahms/gormxray
```

Ensure you have:
- **Go 1.18+**
- **GORM v2**
- **AWS X-Ray SDK for Go** properly configured in your environment.

## Getting Started

Before using the plugin, set up AWS X-Ray. Typically, you’ll run the X-Ray daemon or utilize an AWS environment (like EC2 or ECS) where X-Ray is already integrated. You should begin a root segment in your request or operation handler, so that all subsequent queries can be traced under it.

### Basic Example

```go
package main

import (
    "context"
    "log"

    "github.com/aws/aws-xray-sdk-go/xray"
    "github.com/grahms/gormxray"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
)

func main() {
    // Begin a root segment for an operation (e.g., handling a request)
    ctx, rootSeg := xray.BeginSegment(context.Background(), "UserQueryOperation")
    defer rootSeg.Close(nil)

    // Connect to an in-memory SQLite database
    db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
    if err != nil {
        log.Fatalf("failed to connect database: %v", err)
    }

    // Integrate the plugin with GORM
    if err := db.Use(gormxray.NewPlugin()); err != nil {
        log.Fatalf("failed to register xray plugin: %v", err)
    }

    // Attach the traced context to all DB operations
    db = db.WithContext(ctx)

    var val int
    if err := db.Raw("SELECT 42").Scan(&val).Error; err != nil {
        log.Printf("query error: %v", err)
    } else {
        log.Printf("Query returned: %d", val)
    }

    // In the X-Ray console, you will see a subsegment for this query under "UserQueryOperation"
}
```

### Integrating with an API Handler

If you’re building an API, you often have an incoming request with its own `context.Context`. By using `db.WithContext(ctx)`, each query will be associated with the main segment created for that request, ensuring that your entire request trace is captured end-to-end.

```go
import (
    "context"
    "log"
    "net/http"

    "github.com/aws/aws-xray-sdk-go/xray"
    "github.com/grahms/gormxray"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
)

func handler(w http.ResponseWriter, r *http.Request) {
    // The incoming request context should already have a main X-Ray segment if you’re using xray.Handler
    ctx := r.Context()

    // Connect to the DB (in production, you'd reuse a persistent connection)
    db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
    if err != nil {
        http.Error(w, "failed to connect database", http.StatusInternalServerError)
        return
    }

    // Register the X-Ray plugin
    if err := db.Use(gormxray.NewPlugin()); err != nil {
        http.Error(w, "failed to register xray plugin", http.StatusInternalServerError)
        return
    }

    // Associate the request’s context with DB operations
    db = db.WithContext(ctx)

    // Each query now appears as a subsegment under the main request segment in X-Ray
    if err := db.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)").Error; err != nil {
        log.Printf("error creating table: %v", err)
    }

    if err := db.Exec("INSERT INTO users (name) VALUES ('Alice'), ('Bob')").Error; err != nil {
        log.Printf("error inserting data: %v", err)
    }

    var names []string
    if err := db.Raw("SELECT name FROM users").Scan(&names).Error; err != nil {
        log.Printf("error querying users: %v", err)
    } else {
        log.Printf("queried users: %v", names)
    }

    w.Write([]byte("Data queried successfully"))
}

func main() {
    // Wrap your handler with xray.Handler to ensure each request has its own segment
    http.Handle("/", xray.Handler(xray.NewFixedSegmentNamer("MyService"), http.HandlerFunc(handler)))
    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

### Advanced Options

You can customize the plugin’s behavior with functional options:

- **Exclude Query Variables:** Hide parameter values from metadata.
- **Query Formatter:** Redact sensitive information or pretty-print SQL queries.

```go
db.Use(
    gormxray.NewPlugin(
        gormxray.WithExcludeQueryVars(true),
        gormxray.WithQueryFormatter(func(q string) string {
            return redactNumbers(q)
        }),
    ),
)
```

Where `redactNumbers` might be a function like:

```go
import "regexp"

func redactNumbers(query string) string {
    return regexp.MustCompile(`\d+`).ReplaceAllString(query, "?")
}
```

### Handling Errors

The plugin automatically marks subsegments with errors for failing queries. Non-critical issues like `sql.ErrNoRows` or `gorm.ErrRecordNotFound` are considered normal and won’t degrade the segment’s status.

## Testing

Run unit tests to ensure correctness and stability:

```bash
go test ./...
```

These tests verify that:
- The plugin registers GORM callbacks correctly.
- Subsegments are created for each query.
- Errors and non-critical conditions are handled gracefully.

## Troubleshooting

- **No Subsegments in X-Ray Console:** Ensure a main segment is started (e.g., via `xray.BeginSegment`) before running queries. If using HTTP handlers, wrap them with `xray.Handler`.
- **Missing Metadata:** Confirm that `db.Use(gormxray.NewPlugin())` is called before any queries and that `db.WithContext(ctx)` is applied if you want tracing tied to a specific context.
- **Performance Concerns:** The overhead is minimal. If you have an extremely high query volume, consider adjusting sampling rules or limiting instrumentation in certain critical paths.

## Contributing

Contributions are welcome!
1. Fork the repository.
2. Create a feature branch (`git checkout -b feature/my-improvement`).
3. Implement changes, add tests, and run all tests to ensure stability.
4. Open a pull request with a clear description of your changes.

## License

This project is licensed under the [MIT License](./LICENSE).

## Acknowledgements

- Built upon the foundation of [GORM](https://gorm.io/) and [AWS X-Ray SDK for Go](https://github.com/aws/aws-xray-sdk-go).
- Inspired by developers needing better insight into how database operations impact their service performance.
