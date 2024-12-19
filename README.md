# GORM X-Ray Plugin

[![Go Reference](https://pkg.go.dev/badge/github.com/grahms/gormxray.svg)](https://pkg.go.dev/github.com/grahms/gormxray)
[![Go Report Card](https://goreportcard.com/badge/github.com/grahms/gormxray)](https://goreportcard.com/report/github.com/grahms/gormxray)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](./LICENSE)

**GORM X-Ray Plugin** seamlessly integrates [AWS X-Ray](https://aws.amazon.com/xray/) with the [GORM](https://gorm.io/) ORM, enabling you to trace and visualize database operations. It automatically creates and annotates subsegments for SQL queries, providing deep insights into performance characteristics, bottlenecks, and anomalies within your data access layer.

## Key Features

- **Automatic Tracing:** Hooks into GORM’s lifecycle to trace queries (CREATE, SELECT, UPDATE, DELETE, RAW) without manual instrumentation.
- **Detailed Metadata:** Captures query statements, operation type, table names, and affected rows as X-Ray metadata.
- **Error Annotation:** Records errors and exceptions in X-Ray subsegments, streamlining troubleshooting and root-cause analysis.
- **Customizable Query Formatting:** Redact sensitive data or prettify SQL queries before they’re recorded.
- **Lightweight & Efficient:** Minimal overhead, allowing you to monitor performance in production environments.

## Installation

```bash
go get github.com/grahms/gormxray
```

This will add `gormxray` to your `go.mod` as a dependency. Make sure you have Go 1.18+ and have set up X-Ray in your environment.

## Quick Start

### Prerequisites

- A running AWS X-Ray daemon or X-Ray setup in your environment.
- GORM v2 and a supported database driver.
- A root X-Ray segment started in your request handling logic (e.g., `xray.BeginSegment`).

### Example Usage

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
    // Begin a root segment for the request or operation
    ctx, rootSeg := xray.BeginSegment(context.Background(), "UserQueryOperation")
    defer rootSeg.Close(nil)

    // Connect to an in-memory SQLite DB
    db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
    if err != nil {
        log.Fatalf("failed to connect database: %v", err)
    }

    // Integrate the X-Ray plugin with GORM
    if err := db.Use(gormxray.NewPlugin()); err != nil {
        log.Fatalf("failed to register xray plugin: %v", err)
    }

    // Run a query under the traced context
    db = db.WithContext(ctx)

    var val int
    if err := db.Raw("SELECT 42").Scan(&val).Error; err != nil {
        log.Printf("query error: %v", err)
    } else {
        log.Printf("Query returned: %d", val)
    }

    // Check your AWS X-Ray console to see the traced segments and metadata
}
```

**How It Works:**  
Once integrated, every GORM operation runs inside an X-Ray subsegment. The plugin adds metadata such as the executed SQL query, the operation type (e.g., SELECT, UPDATE), and how many rows were affected. If an error occurs, it will also annotate the subsegment, helping you quickly diagnose issues.

### Advanced Configuration

Use functional options to customize the plugin’s behavior, such as excluding query variables or formatting queries before recording them:

```go
db.Use(
    gormxray.NewPlugin(
        gormxray.WithExcludeQueryVars(true),
        gormxray.WithQueryFormatter(func(q string) string {
            // Redact numbers to prevent sensitive info leakage
            return redactNumbers(q)
        }),
    ),
)
```

Where `redactNumbers` might look like:

```go
import "regexp"

func redactNumbers(query string) string {
    return regexp.MustCompile(`\d+`).ReplaceAllString(query, "?")
}
```

**Available Options:**

- `WithExcludeQueryVars(bool)`: If `true`, does not expand query variables into the SQL statement, keeping it parameterized.
- `WithQueryFormatter(func(string) string)`: Supplies a custom function to modify queries before they’re recorded as metadata.

### Handling Non-Critical Errors

The plugin gracefully ignores common non-critical errors like `sql.ErrNoRows` or `gorm.ErrRecordNotFound`. These do not mark your segments as erroneous but still record the query details. Critical errors, however, are logged in X-Ray, simplifying root-cause analysis.

## Testing

A test suite is included to ensure the plugin’s correctness and stability. The tests run an in-memory SQLite database and verify that:

- The plugin registers callbacks correctly.
- Queries produce subsegments.
- Non-critical errors are ignored, while real errors are captured.

Run the tests with:

```bash
go test ./...
```

## Troubleshooting

- **No Traces in X-Ray Console:**  
  Ensure you’ve started a root segment before running queries and that the X-Ray daemon or service is properly configured.

- **No Metadata in Subsegments:**  
  Confirm that the plugin is successfully registered via `db.Use(...)` before any queries are performed.

- **Performance Concerns:**  
  The overhead should be minimal. If you’re dealing with extremely high query volumes, consider adjusting your sampling rules or focusing instrumentation on critical code paths.

## Contributing

Contributions are welcome. To contribute:

1. Fork the repository and create a feature branch.
2. Add tests for any changes you introduce.
3. Submit a pull request with a clear description of the improvements.

## License

This project is licensed under the [MIT License](./LICENSE).

## Acknowledgements

- Built on the foundational work of [GORM](https://gorm.io/) and the [AWS X-Ray SDK for Go](https://github.com/aws/aws-xray-sdk-go).
- Inspired by developers seeking greater observability and performance insights into database operations within their distributed, microservice-driven architectures.
```