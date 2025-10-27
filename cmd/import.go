package cmd

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"os"

	"github.com/lib/pq"
	"github.com/spf13/cobra"
)

var importProfile, importFile, importTable string

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import CSV file into PostgreSQL table",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, _ := loadConfig()
		p := cfg.Profiles[importFile]
		db, _ := sql.Open("postgres", p.DbURL)
		defer db.Close()

		file, _ := os.Open(importFile)
		defer file.Close()
		reader := csv.NewReader(file)
		cols, _ := reader.Read()

		tx, _ := db.Begin()
		stmt, _ := tx.Prepare(fmt.Sprintf(`COPY %s (%s) FROM STDIN WITH (FORMAT csv)`, importTable, pq.QuoteIdentifier(cols[0])))
		defer stmt.Close()

		for {
			record, err := reader.Read()
			if err != nil {
				break
			}
			stmt.Exec(record)
		}
		tx.Commit()
		fmt.Println("✅ Import completed.")
	},
}

func init() {
	importCmd.Flags().StringVar(&importProfile, "profile", "", "Profile name")
	importCmd.Flags().StringVar(&importFile, "file", "", "CSV file")
	importCmd.Flags().StringVar(&importTable, "table", "", "Target table")
	rootCmd.AddCommand(importCmd)
}
