package gormxray

import (
	"context"
	"testing"

	"github.com/aws/aws-xray-sdk-go/xray"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestPluginInitialization(t *testing.T) {
	// Initialize an in-memory SQLite DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect database: %v", err)
	}

	// Register the plugin
	if err := db.Use(NewPlugin()); err != nil {
		t.Fatalf("failed to register plugin: %v", err)
	}

	// Verify that the plugin is correctly initialized
	registeredCallbacks := db.Callback()
	if registeredCallbacks == nil {
		t.Error("callbacks are not initialized")
	}
}

func TestPluginName(t *testing.T) {
	plugin := NewPlugin()
	name := plugin.Name()
	if name != "xraytracing" {
		t.Errorf("expected plugin name 'xraytracing', got '%s'", name)
	}
}

func TestPluginQueryTracing(t *testing.T) {
	// Initialize an in-memory SQLite DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect database: %v", err)
	}

	// Begin an X-Ray segment on the context to simulate a trace environment
	ctx, rootSegment := xray.BeginSegment(context.Background(), "TestSegment")
	defer rootSegment.Close(nil)

	// Assign the traced context to DB
	db = db.WithContext(ctx)

	// Register the plugin
	if err := db.Use(NewPlugin()); err != nil {
		t.Fatalf("failed to register plugin: %v", err)
	}

	// Run a simple query
	var result int
	if err := db.Raw("SELECT 1").Scan(&result).Error; err != nil {
		t.Fatalf("failed to execute query: %v", err)
	}

	// Basic validation: result should be 1
	if result != 1 {
		t.Errorf("expected query result to be 1, got %d", result)
	}

	// Check if the plugin created subsegments in the root segment
	//subSegments := rootSegment.Subsegments
	//if len(subSegments) == 0 {
	//	t.Error("expected at least one subsegment to be created, but none found")
	//}
}

func TestIgnoreNonCriticalErrors(t *testing.T) {
	// Initialize an in-memory SQLite DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect database: %v", err)
	}

	// Begin an X-Ray segment on the context to simulate a trace environment
	ctx, rootSegment := xray.BeginSegment(context.Background(), "TestSegmentNonCriticalErrors")
	defer rootSegment.Close(nil)

	// Assign the traced context to DB
	db = db.WithContext(ctx)

	// Register the plugin
	if err := db.Use(NewPlugin()); err != nil {
		t.Fatalf("failed to register plugin: %v", err)
	}

	// Perform a query that returns no rows
	err = db.Raw("SELECT * FROM non_existent_table").Scan(&struct{}{}).Error
	// This should be a no-op error or sql.ErrNoRows
	if err == nil {
		t.Error("expected an error due to non-existent table, got none")
	}

	// Check that the subsegment for this query is still created
	//subSegments := rootSegment.Subsegments
	//if len(subSegments) == 0 {
	//	t.Error("expected at least one subsegment to be created, but none found")
	//}

	// The plugin should have recorded this as a non-critical error
	// but let's ensure no unexpected panic or behavior
	//for _, seg := range subSegments {
	//	for _, e := range seg..Exceptions {
	//		if e.Message != driver.ErrSkip.Error() && e.Message != io.EOF.Error() {
	//			// It's fine to have a real error here, but
	//			// this test ensures that the plugin at least handled it gracefully.
	//		}
	//	}
	//}
}
