package main

import (
	"fmt"
	"log"

	"github.com/glebarez/sqlite"
	"github.com/wwwzy/CentAgent/internal/storage"
	"gorm.io/gorm"
)

func main() {
	// Connect to the database
	db, err := gorm.Open(sqlite.Open("centagent.db"), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	fmt.Println("--- Verifying CentAgent Database ---")

	// Verify ContainerStats
	var statsCount int64
	// We need to verify if the table exists first to avoid panic if migration didn't run
	if !db.Migrator().HasTable(&storage.ContainerStat{}) {
		fmt.Println("Table 'container_stats' does not exist yet.")
	} else {
		db.Model(&storage.ContainerStat{}).Count(&statsCount)
		fmt.Printf("Total Container Stats Records: %d\n", statsCount)

		if statsCount > 0 {
			var stats []storage.ContainerStat
			db.Order("collected_at desc").Limit(5).Find(&stats)
			fmt.Println("Latest 5 Stats (Local Time):")
			for _, s := range stats {
				fmt.Printf("  [%s] %s CPU:%.2f%% Mem:%.2f%%\n",
					s.CollectedAt.Local().Format("2006-01-02 15:04:05"), s.ContainerName, s.CPUPercent, s.MemPercent)
			}
		}
	}

	fmt.Println("\n------------------------------------")

	// Verify ContainerLogs
	var logsCount int64
	if !db.Migrator().HasTable(&storage.ContainerLog{}) {
		fmt.Println("Table 'container_logs' does not exist yet.")
	} else {
		db.Model(&storage.ContainerLog{}).Count(&logsCount)
		fmt.Printf("Total Container Log Records: %d\n", logsCount)

		if logsCount > 0 {
			var logs []storage.ContainerLog
			db.Order("timestamp desc").Limit(5).Find(&logs)
			fmt.Println("Latest 5 Logs (Local Time):")
			for _, l := range logs {
				msg := l.Message
				if len(msg) > 50 {
					msg = msg[:47] + "..."
				}
				fmt.Printf("  [%s] %s [%s] %s\n",
					l.Timestamp.Local().Format("2006-01-02 15:04:05"), l.ContainerName, l.Source, msg)
			}
		}
	}
}
