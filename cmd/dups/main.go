package main

import (
	"crypto/rand"
	"crypto/sha1"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	_ "github.com/mattn/go-sqlite3"
)

var (
	flLoadMap  = flag.String("l", "", "load existing map from file (JSON format)")
	flSaveMap  = flag.String("o", "", "file to save map of file hashes to (JSON format) - empty means no output")
	flDB       = flag.String("db", "", "sqlite3 database file for input/output (primary storage)")
	flImport   = flag.String("import-json", "", "import hash map from JSON file into database (requires -db)")
	flExport   = flag.String("export-json", "", "export hash map from database to JSON file (requires -db)")
	flWorkers  = flag.Int("w", runtime.NumCPU(), "number of workers for measurements")
	flHardlink = flag.Bool("H", false, "hardlink the duplicate files")
	flSymlink  = flag.Bool("s", false, "symlink the duplicate files")
	flQuiet    = flag.Bool("q", false, "less output")
	flVerbose  = flag.Bool("v", false, "more output")
	nprocs     = 1
)

func init() {
	nprocs = runtime.NumCPU()
	runtime.GOMAXPROCS(nprocs)
}

func main() {
	flag.Parse()

	found := map[string]string{}
	if len(*flLoadMap) > 0 {
		fh, err := os.Open(*flLoadMap)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		buf, err := io.ReadAll(fh)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if err = json.Unmarshal(buf, &found); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	// Check if we're importing a JSON file into the database
	if *flImport != "" {
		if *flDB == "" {
			fmt.Fprintln(os.Stderr, "Error: -import-json requires -db to be specified")
			os.Exit(1)
		}

		var err error
		db, err := sql.Open("sqlite3", *flDB)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error opening database:", err)
			os.Exit(1)
		}
		defer db.Close()

		// Create table if it doesn't exist
		sqlStmt := `CREATE TABLE IF NOT EXISTS file_hashes (
			id INTEGER PRIMARY KEY,
			hash TEXT NOT NULL UNIQUE,
			file_path TEXT NOT NULL,
			size INTEGER,
			modified_time DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_hash ON file_hashes(hash);
		CREATE INDEX IF NOT EXISTS idx_file_path ON file_hashes(file_path);`

		_, err = db.Exec(sqlStmt)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error creating table:", err)
			os.Exit(1)
		}

		// Load the JSON file
		jsonFile, err := os.Open(*flImport)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error opening JSON file:", err)
			os.Exit(1)
		}
		defer jsonFile.Close()

		buf, err := io.ReadAll(jsonFile)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error reading JSON file:", err)
			os.Exit(1)
		}

		var importedMap map[string]string
		if err = json.Unmarshal(buf, &importedMap); err != nil {
			fmt.Fprintln(os.Stderr, "Error parsing JSON file:", err)
			os.Exit(1)
		}

		// Import the data into the database
		tx, err := db.Begin()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error beginning transaction:", err)
			os.Exit(1)
		}

		stmt, err := tx.Prepare("INSERT OR REPLACE INTO file_hashes (hash, file_path) VALUES (?, ?)")
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error preparing statement:", err)
			os.Exit(1)
		}
		defer stmt.Close()

		count := 0
		for hash, path := range importedMap {
			_, err = stmt.Exec(hash, path)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error inserting record:", err)
				continue
			}
			count++
		}

		if err = tx.Commit(); err != nil {
			fmt.Fprintln(os.Stderr, "Error committing transaction:", err)
			os.Exit(1)
		}

		fmt.Printf("Successfully imported %d records from %s into database %s\n", count, *flImport, *flDB)
		return // Exit early after import
	}

	// Check if we're exporting the database to JSON
	if *flExport != "" {
		if *flDB == "" {
			fmt.Fprintln(os.Stderr, "Error: -export-json requires -db to be specified")
			os.Exit(1)
		}

		db, err := sql.Open("sqlite3", *flDB)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error opening database:", err)
			os.Exit(1)
		}
		defer db.Close()

		// Query all records from the database
		rows, err := db.Query("SELECT hash, file_path FROM file_hashes")
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error querying database:", err)
			os.Exit(1)
		}
		defer rows.Close()

		exportMap := make(map[string]string)
		for rows.Next() {
			var hash, filePath string
			err = rows.Scan(&hash, &filePath)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error scanning row:", err)
				continue
			}
			exportMap[hash] = filePath
		}

		// Write the map to the JSON file
		jsonFile, err := os.Create(*flExport)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error creating JSON file:", err)
			os.Exit(1)
		}
		defer jsonFile.Close()

		encoder := json.NewEncoder(jsonFile)
		encoder.SetIndent("", "  ")
		if err = encoder.Encode(exportMap); err != nil {
			fmt.Fprintln(os.Stderr, "Error encoding JSON:", err)
			os.Exit(1)
		}

		fmt.Printf("Successfully exported %d records from database %s to %s\n", len(exportMap), *flDB, *flExport)
		return // Exit early after export
	}

	// Initialize database if provided (but not for import)
	var db *sql.DB
	if *flDB != "" {
		var err error
		db, err = sql.Open("sqlite3", *flDB)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error opening database:", err)
			os.Exit(1)
		}
		defer db.Close()

		// Create table if it doesn't exist
		sqlStmt := `CREATE TABLE IF NOT EXISTS file_hashes (
			id INTEGER PRIMARY KEY,
			hash TEXT NOT NULL UNIQUE,
			file_path TEXT NOT NULL,
			size INTEGER,
			modified_time DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_hash ON file_hashes(hash);
		CREATE INDEX IF NOT EXISTS idx_file_path ON file_hashes(file_path);`

		_, err = db.Exec(sqlStmt)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error creating table:", err)
			os.Exit(1)
		}

		// Load existing data from database if no load map was provided
		if len(*flLoadMap) == 0 {
			rows, err := db.Query("SELECT hash, file_path FROM file_hashes")
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error querying database:", err)
				os.Exit(1)
			}
			defer rows.Close()

			for rows.Next() {
				var hash, filePath string
				err = rows.Scan(&hash, &filePath)
				if err != nil {
					fmt.Fprintln(os.Stderr, "Error scanning row:", err)
					continue
				}
				found[hash] = filePath
			}
		}
	}

	for _, arg := range flag.Args() {
		savings := int64(0)

		workers := make(chan int, *flWorkers)
		mu := sync.Mutex{}

		// Channel for sending measurements to database
		type measurement struct {
			hash    string
			path    string
			size    int64
		}
		measurements := make(chan measurement, *flWorkers*2) // Buffered channel

		// Start database writer goroutine if DB is provided
		var wgDB sync.WaitGroup
		if db != nil {
			wgDB.Add(1)
			go func() {
				defer wgDB.Done()
				for m := range measurements {
					// Insert or update the record in the database
					stmt, err := db.Prepare("INSERT OR REPLACE INTO file_hashes (hash, file_path, size) VALUES (?, ?, ?)")
					if err != nil {
						fmt.Fprintln(os.Stderr, "Error preparing statement:", err)
						continue
					}
					_, err = stmt.Exec(m.hash, m.path, m.size)
					if err != nil {
						fmt.Fprintln(os.Stderr, "Error inserting record:", err)
					}
					stmt.Close()
				}
			}()
		}

		// WaitGroup to track all worker goroutines
		var wgWorkers sync.WaitGroup

		err := filepath.Walk(arg, func(path string, info os.FileInfo, err error) error {
			/*
				if err != nil {
					return err
				}
			*/
			if !info.Mode().IsRegular() {
				return nil
			}
			workers <- 1
			wgWorkers.Add(1)
			go func() {
				defer wgWorkers.Done()
				defer func() { <-workers }()

				fh, err := os.Open(path)
				if err != nil {
					fmt.Fprintln(os.Stderr, err, path)
					return
				}
				defer fh.Close()

				h := sha1.New()
				if _, err = io.Copy(h, fh); err != nil {
					fmt.Fprintln(os.Stderr, err, path)
					return
				}
				sum := fmt.Sprintf("%x", h.Sum(nil))

				// get the absolute filename
				p, err := filepath.Abs(path)
				if err != nil {
					fmt.Fprintln(os.Stderr, err, path)
					return
				}
				path = p
				if *flVerbose {
					fmt.Printf("SHA1(%s)= %s\n", path, sum)
				}

				mu.Lock()
				defer mu.Unlock()
				if fpath, ok := found[sum]; ok && fpath != path {
					if !(*flQuiet) {
						fmt.Printf("%q is the same content as %q\n", path, fpath)
					}
					if *flHardlink {
						if err = SafeLink(fpath, path, true); err != nil {
							fmt.Fprintln(os.Stderr, err, path)
							return
						}
						fmt.Printf("hard linked %q to %q\n", path, fpath)
					}
					if *flSymlink {
						if err = SafeLink(fpath, path, false); err != nil {
							fmt.Fprintln(os.Stderr, err, path)
							return
						}
						fmt.Printf("soft linked %q to %q\n", path, fpath)
					}
					savings += info.Size()
				} else {
					found[sum] = path
				}

				// Send measurement to database if DB is provided
				if db != nil {
					measurements <- measurement{
						hash: sum,
						path: path,
						size: info.Size(),
					}
				}
			}()
			return nil
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		// Wait for all workers to finish
		wgWorkers.Wait()

		// Close the measurements channel and wait for DB writer to finish
		if db != nil {
			close(measurements)
			wgDB.Wait()
		}
		fmt.Printf("Savings of %fmb\n", float64(savings)/1024.0/1024.0)

		// Only write the JSON file if the -o flag is specified with a non-empty value
		if *flSaveMap != "" {
			fh, err := os.Create(*flSaveMap)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			buf, err := json.Marshal(found)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			_, err = fh.Write(buf)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			fh.Close()
			fmt.Fprintf(os.Stderr, "wrote %q\n", fh.Name())
		}
	}
}

// SafeLink overrides newname if it already exists. If there is an error in creating the link, the transaction is rolled back
func SafeLink(oldname, newname string, hard bool) error {
	var backupName string
	// check if newname exists
	if fi, err := os.Stat(newname); err == nil && fi != nil {
		// make a random name
		buf := make([]byte, 5)
		if _, err = rand.Read(buf); err != nil {
			return err
		}
		backupName = fmt.Sprintf("%s.%x", newname, buf)
		// move newname to the random name backupName
		if err = os.Rename(newname, backupName); err != nil {
			return err
		}
	}
	if hard {
		// hardlink oldname to newname
		if err := os.Link(oldname, newname); err != nil {
			// if that failed, and there is a backupName
			if len(backupName) > 0 {
				// then move back the backup
				if err = os.Rename(backupName, newname); err != nil {
					return err
				}
			}
			return err
		}
	} else {
		// symlink
		relpath, err := filepath.Rel(filepath.Dir(newname), oldname)
		if err != nil {
			return err
		}
		if err := os.Symlink(relpath, newname); err != nil {
			// if that failed, and there is a backupName
			if len(backupName) > 0 {
				// then move back the backup
				if err = os.Rename(backupName, newname); err != nil {
					return err
				}
			}
			return err
		}
	}
	// remove the backupName
	if len(backupName) > 0 {
		os.Remove(backupName)
	}
	return nil
}
