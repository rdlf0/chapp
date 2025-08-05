package main

import (
	"flag"
	"fmt"
	"log"

	"chapp/pkg/database"
)

func main() {
	var (
		dbPath     = flag.String("db", "chapp.db", "Database file path")
		backupPath = flag.String("backup", "", "Backup file path")
		stats      = flag.Bool("stats", false, "Show database statistics")
		cleanup    = flag.Bool("cleanup", false, "Cleanup expired sessions")
	)
	flag.Parse()

	// Initialize database
	db, err := database.NewSQLite(*dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Handle different operations
	if *backupPath != "" {
		if err := database.BackupDatabase(*dbPath, *backupPath); err != nil {
			log.Fatalf("Failed to backup database: %v", err)
		}
		fmt.Printf("Database backed up to %s\n", *backupPath)
	}

	if *stats {
		stats, err := database.GetDatabaseStats(db)
		if err != nil {
			log.Fatalf("Failed to get database stats: %v", err)
		}
		fmt.Printf("Database Statistics:\n")
		fmt.Printf("  Users: %d\n", stats.UserCount)
		fmt.Printf("  Sessions: %d\n", stats.SessionCount)
		fmt.Printf("  Credentials: %d\n", stats.CredentialCount)
	}

	if *cleanup {
		if err := database.CleanupDatabase(db); err != nil {
			log.Fatalf("Failed to cleanup database: %v", err)
		}
		fmt.Println("Database cleanup completed")
	}

	// If no flags provided, show usage
	if !*stats && !*cleanup && *backupPath == "" {
		fmt.Println("Chapp Database Management Tool")
		fmt.Println("Usage:")
		flag.PrintDefaults()
		fmt.Println("\nExamples:")
		fmt.Println("  go run scripts/db_manage.go -stats")
		fmt.Println("  go run scripts/db_manage.go -cleanup")
		fmt.Println("  go run scripts/db_manage.go -backup backup.db")
	}
}
