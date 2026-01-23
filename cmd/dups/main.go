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
	"strings"
	"syscall"
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
	flHardlinkPaths = flag.String("H-paths", "", "comma-separated list of allowed paths for hardlinking (if specified, only hardlink within these paths)")
	flSymlink  = flag.Bool("s", false, "symlink the duplicate files")
	flQuiet    = flag.Bool("q", false, "less output")
	flVerbose  = flag.Bool("v", false, "more output")
	nprocs     = 1
)

// isPathAllowed checks if a path is within any of the allowed paths
func isPathAllowed(path string, allowedPaths []string) bool {
	if len(allowedPaths) == 0 {
		// If no allowed paths specified, all paths are allowed
		return true
	}

	// Clean the path for comparison
	cleanPath := filepath.Clean(path)

	for _, allowedPath := range allowedPaths {
		// Convert allowed path to absolute path if it's not already
		absAllowedPath := allowedPath
		if !filepath.IsAbs(allowedPath) {
			var err error
			absAllowedPath, err = filepath.Abs(allowedPath)
			if err != nil {
				continue
			}
		}

		// Check if the path is within the allowed path
		rel, err := filepath.Rel(absAllowedPath, cleanPath)
		if err != nil {
			continue
		}
		// If the relative path doesn't start with "..", it's within the allowed path
		if !strings.HasPrefix(rel, "..") {
			return true
		}
	}
	return false
}

func init() {
	nprocs = runtime.NumCPU()
	runtime.GOMAXPROCS(nprocs)
}

func main() {
	flag.Parse()

	// Parse allowed hardlink paths if specified
	var allowedHardlinkPaths []string
	if *flHardlinkPaths != "" {
		paths := strings.Split(*flHardlinkPaths, ",")
		// Clean up and convert paths to absolute
		for _, path := range paths {
			cleanPath := filepath.Clean(strings.TrimSpace(path))
			if !filepath.IsAbs(cleanPath) {
				absPath, err := filepath.Abs(cleanPath)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error converting path to absolute: %s, %v\n", cleanPath, err)
					continue
				}
				allowedHardlinkPaths = append(allowedHardlinkPaths, absPath)
			} else {
				allowedHardlinkPaths = append(allowedHardlinkPaths, cleanPath)
			}
		}
	}

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
			hash TEXT NOT NULL,
			file_path TEXT NOT NULL UNIQUE,
			device_id TEXT,  -- Store device ID as string (major:minor)
			size INTEGER,
			modified_time DATETIME,
			checked_time DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_hash ON file_hashes(hash);
		CREATE INDEX IF NOT EXISTS idx_file_path ON file_hashes(file_path);
		CREATE INDEX IF NOT EXISTS idx_device_id ON file_hashes(device_id);
		CREATE INDEX IF NOT EXISTS idx_checked_time ON file_hashes(checked_time);`

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

		stmt, err := tx.Prepare("INSERT OR IGNORE INTO file_hashes (hash, file_path, device_id) VALUES (?, ?, ?)")
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error preparing statement:", err)
			os.Exit(1)
		}
		defer stmt.Close()

		count := 0
		for hash, path := range importedMap {
			// Get device ID for the file
			var deviceId string
			stat, err := os.Stat(path)
			if err != nil {
				// If file doesn't exist, skip device ID
				deviceId = ""
			} else {
				sysStat, ok := stat.Sys().(*syscall.Stat_t)
				if ok {
					// Use major device number only (not inode number)
					majorDev := (sysStat.Dev >> 8) & 0xff | ((sysStat.Dev >> 32) & 0xfff00) // Extract major device number
					deviceId = fmt.Sprintf("%d", majorDev)
				} else {
					deviceId = ""
				}
			}
			_, err = stmt.Exec(hash, path, deviceId)
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
			hash TEXT NOT NULL,
			file_path TEXT NOT NULL UNIQUE,
			device_id TEXT,  -- Store device ID as string (major:minor)
			size INTEGER,
			modified_time DATETIME,
			checked_time DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_hash ON file_hashes(hash);
		CREATE INDEX IF NOT EXISTS idx_file_path ON file_hashes(file_path);
		CREATE INDEX IF NOT EXISTS idx_device_id ON file_hashes(device_id);
		CREATE INDEX IF NOT EXISTS idx_checked_time ON file_hashes(checked_time);`

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
					// Insert the record in the database (ignore if file_path already exists)
					stmt, err := db.Prepare("INSERT OR IGNORE INTO file_hashes (hash, file_path, device_id, size, modified_time) VALUES (?, ?, ?, ?, ?)")
					if err != nil {
						fmt.Fprintln(os.Stderr, "Error preparing statement:", err)
						continue
					}
					// Get file modification time and device ID
					info, err := os.Stat(m.path)
					var modTime string
					var deviceId string
					if err != nil {
						modTime = ""
						deviceId = ""
					} else {
						modTime = info.ModTime().Format("2006-01-02 15:04:05")
						sysStat, ok := info.Sys().(*syscall.Stat_t)
						if ok {
							// Use major device number only (not inode number)
							majorDev := (sysStat.Dev >> 8) & 0xff | ((sysStat.Dev >> 32) & 0xfff00) // Extract major device number
							deviceId = fmt.Sprintf("%d", majorDev)
						} else {
							deviceId = ""
						}
					}
					_, err = stmt.Exec(m.hash, m.path, deviceId, m.size, modTime)
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

				// Get the absolute filename
				absPath, err := filepath.Abs(path)
				if err != nil {
					fmt.Fprintln(os.Stderr, err, path)
					return
				}

				// Check if this file path already exists in the database
				var existingHash string
				var existingSize int64
				var existingDeviceId string
				if db != nil {
					row := db.QueryRow("SELECT hash, size, device_id FROM file_hashes WHERE file_path = ?", absPath)
					err = row.Scan(&existingHash, &existingSize, &existingDeviceId)
					if err == nil {
						// File exists in database, check if size has changed
						if existingSize == info.Size() {
							// File size hasn't changed, assume content is the same (skip checksum)
							if *flVerbose {
								fmt.Printf("SKIPPED checksum for %s (already in DB, size unchanged: %d bytes)\n", absPath, existingSize)
							}

							// Get current device ID
							sysStat, ok := info.Sys().(*syscall.Stat_t)
							if !ok {
								// If we can't get device info, skip hardlinking
								if *flVerbose {
									fmt.Printf("Skipped hardlink: could not get device info for %s\n", absPath)
								}
							}
							var currentMajorDev uint64
							if ok {
								// Extract major device number
								currentMajorDev = (sysStat.Dev >> 8) & 0xff | ((sysStat.Dev >> 32) & 0xfff00)
							}

							mu.Lock()
							defer mu.Unlock()
							if fpath, ok := found[existingHash]; ok && fpath != absPath {
								if !(*flQuiet) {
									fmt.Printf("%q is the same content as %q\n", absPath, fpath)
								}
								if *flHardlink && ok { // Only proceed if we have sysStat
									// Check if both files are on the same device before hardlinking
									var targetSysStat syscall.Stat_t
									targetInfo, err := os.Stat(fpath)
									if err == nil && targetInfo != nil {
										if targetSys, ok := targetInfo.Sys().(*syscall.Stat_t); ok {
											targetSysStat = *targetSys
											// Extract major device numbers
											targetMajorDev := (targetSysStat.Dev >> 8) & 0xff | ((targetSysStat.Dev >> 32) & 0xfff00)
											// Only hardlink if both files are on the same device
											if currentMajorDev == targetMajorDev {
												// Check if both files are within allowed paths (if specified)
												if isPathAllowed(absPath, allowedHardlinkPaths) && isPathAllowed(fpath, allowedHardlinkPaths) {
													if err = SafeLink(fpath, absPath, true); err != nil {
														fmt.Fprintln(os.Stderr, err, absPath)
														return
													}
													fmt.Printf("hard linked %q to %q\n", absPath, fpath)
												} else {
													if *flVerbose {
														fmt.Printf("Skipped hardlink: file(s) not in allowed paths (current: %s, target: %s)\n",
															absPath, fpath)
													}
												}
											} else {
												if *flVerbose {
													fmt.Printf("Skipped hardlink: files on different devices (%d vs %d)\n",
														int(currentMajorDev), int(targetMajorDev))
												}
											}
										}
									} else {
										// If we can't get target stats, skip hardlinking
										if *flVerbose {
											fmt.Printf("Skipped hardlink: could not stat target file %s\n", fpath)
										}
									}
								} else if *flHardlink && !ok {
									// If we couldn't get device info, skip hardlinking
									if *flVerbose {
										fmt.Printf("Skipped hardlink: could not get device info for %s\n", absPath)
									}
								}
								if *flSymlink {
									if err = SafeLink(fpath, absPath, false); err != nil {
										fmt.Fprintln(os.Stderr, err, absPath)
										return
									}
									fmt.Printf("soft linked %q to %q\n", absPath, fpath)
								}
								savings += info.Size()
							} else {
								found[existingHash] = absPath
							}

							// Update the checked_time in the database
							_, err = db.Exec("UPDATE file_hashes SET checked_time = CURRENT_TIMESTAMP WHERE file_path = ?", absPath)
							if err != nil && *flVerbose {
								fmt.Fprintf(os.Stderr, "Warning: could not update checked_time for %s: %v\n", absPath, err)
							}
							return
						} else {
							if *flVerbose {
								fmt.Printf("File size changed: %s (%d -> %d bytes)\n",
									absPath, existingSize, info.Size())
							}
						}
					}
				}

				// Calculate hash for new or changed file
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

				if *flVerbose {
					fmt.Printf("SHA1(%s)= %s\n", absPath, sum)
				}

				mu.Lock()
				defer mu.Unlock()
				if fpath, ok := found[sum]; ok && fpath != absPath {
					if !(*flQuiet) {
						fmt.Printf("%q is the same content as %q\n", absPath, fpath)
					}
					if *flHardlink {
						// Check if both files are on the same device before hardlinking
						var targetSysStat syscall.Stat_t
						targetInfo, err := os.Stat(fpath)
						if err == nil && targetInfo != nil {
							if targetSys, ok := targetInfo.Sys().(*syscall.Stat_t); ok {
								targetSysStat = *targetSys
								sysStat, ok := info.Sys().(*syscall.Stat_t)
								if ok {
									// Extract major device numbers
									currentMajorDev := (sysStat.Dev >> 8) & 0xff | ((sysStat.Dev >> 32) & 0xfff00)
									targetMajorDev := (targetSysStat.Dev >> 8) & 0xff | ((targetSysStat.Dev >> 32) & 0xfff00)
									// Only hardlink if both files are on the same device
									if currentMajorDev == targetMajorDev {
										// Check if both files are within allowed paths (if specified)
										if isPathAllowed(absPath, allowedHardlinkPaths) && isPathAllowed(fpath, allowedHardlinkPaths) {
											if err = SafeLink(fpath, absPath, true); err != nil {
												fmt.Fprintln(os.Stderr, err, absPath)
												return
											}
											fmt.Printf("hard linked %q to %q\n", absPath, fpath)
										} else {
											if *flVerbose {
												fmt.Printf("Skipped hardlink: file(s) not in allowed paths (current: %s, target: %s)\n",
													absPath, fpath)
											}
										}
									} else {
										if *flVerbose {
											fmt.Printf("Skipped hardlink: files on different devices (%d vs %d)\n",
												int(currentMajorDev), int(targetMajorDev))
										}
									}
								} else {
									// If we can't get current file stats, skip hardlinking
									if *flVerbose {
										fmt.Printf("Skipped hardlink: could not get device info for %s\n", absPath)
									}
								}
							} else {
								// If we can't get target stats, skip hardlinking
								if *flVerbose {
									fmt.Printf("Skipped hardlink: could not get device info for target file %s\n", fpath)
								}
							}
						} else {
							// If we can't stat target file, skip hardlinking
							if *flVerbose {
								fmt.Printf("Skipped hardlink: could not stat target file %s\n", fpath)
							}
						}
					}
					if *flSymlink {
						if err = SafeLink(fpath, absPath, false); err != nil {
							fmt.Fprintln(os.Stderr, err, absPath)
							return
						}
						fmt.Printf("soft linked %q to %q\n", absPath, fpath)
					}
					savings += info.Size()
				} else {
					found[sum] = absPath
				}

				// Send measurement to database if DB is provided
				if db != nil {
					measurements <- measurement{
						hash: sum,
						path: absPath,
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
