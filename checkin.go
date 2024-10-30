package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"time"
)

func main() {
	// Define command-line flags for the two modes
	scanMode := flag.Bool("scan", false, "Start barcode scanning mode")
	exportMode := flag.Bool("export", false, "Export records within a date or date range (format: YYYY-MM-DD)")
	startDate := flag.String("start", "", "Start date for export (required if using export mode)")
	endDate := flag.String("end", "", "End date for export (optional, for a date range)")
	helpFlag := flag.Bool("help", false, "Display this help message")

	flag.Parse()

	// Display help message if -help is passed or no arguments are given
	if *helpFlag || flag.NFlag() == 0 {
		displayHelp()
		return
	}

	// Determine which mode to run
	if *scanMode {
		runScanMode()
	} else if *exportMode {
		if *startDate == "" {
			fmt.Println("Error: Start date is required for export mode.")
			return
		}
		runExportMode(*startDate, *endDate)
	} else {
		fmt.Println("Error: Please specify either -scan or -export.")
	}
}

// displayHelp prints the help message
func displayHelp() {
	fmt.Println("Barcode Scanner Program")
	fmt.Println("Usage:")
	fmt.Println("  -scan                  : Start barcode scanning mode.")
	fmt.Println("  -export                : Export records within a date or date range.")
	fmt.Println("  -start=<YYYY-MM-DD>    : Specify the start date for export (required if using export mode).")
	fmt.Println("  -end=<YYYY-MM-DD>      : Specify the end date for export (optional, for a date range).")
	fmt.Println("  -help                  : Display this help message.")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  ./checkin -scan")
	fmt.Println("  ./checkin -export -start=2024-10-25")
	fmt.Println("  ./checkin -export -start=2024-10-24 -end=2024-10-26")
	fmt.Println("  ./checkin -help")
}

// runScanMode handles the barcode scanning and saving data to the CSV
func runScanMode() {
	file, err := os.OpenFile("scans.csv", os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		fmt.Println("Error opening/creating file:", err)
		return
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	fmt.Println("Barcode scanner ready. Type 'exit' to quit.")

	// Regular expression to match numeric IDs
	numRegex := regexp.MustCompile(`^\d+$`)

	// Initialize the daily count and load the count for today if it exists
	currentDate := time.Now().Format("2006-01-02")
	dailyCount := getDailyCount(file, currentDate)

	for {
		fmt.Print("Barcode ID: ")
		var barcodeID string
		fmt.Scanln(&barcodeID)

		if barcodeID == "exit" {
			fmt.Println("Exiting scan mode.")
			break
		}

		// Ignore non-numeric IDs
		if !numRegex.MatchString(barcodeID) {
			fmt.Println("Invalid input. Please enter a numeric barcode ID.")
			continue
		}

		// Check if this barcode ID has been scanned within the last 2 hours
		if checkRecentDuplicate(file, barcodeID) {
			fmt.Println("Duplicate entry within 2 hours detected. Skipping entry.")
			continue
		}

		// Update the daily count and check if a new day has started
		now := time.Now()
		if now.Format("2006-01-02") != currentDate {
			// Reset daily count and update the current date
			currentDate = now.Format("2006-01-02")
			dailyCount = 0
		}
		dailyCount++

		// Generate a timestamp in local time zone
		timestamp := now.Format("2006-01-02T15:04:05-07:00")
		record := []string{timestamp, barcodeID, fmt.Sprintf("%d", dailyCount)}
		if err := writer.Write(record); err != nil {
			fmt.Println("Error writing to CSV:", err)
			continue
		}

		writer.Flush()
		if err := writer.Error(); err != nil {
			fmt.Println("Error flushing to CSV:", err)
		} else {
			fmt.Println("Recorded:", record)
		}
	}
}

// getDailyCount reads the CSV and returns the current daily count for the specified date
func getDailyCount(file *os.File, currentDate string) int {
	// Go back to the beginning of the file to read all records
	if _, err := file.Seek(0, 0); err != nil {
		fmt.Println("Error seeking to beginning of file:", err)
		return 0
	}

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		fmt.Println("Error reading CSV:", err)
		return 0
	}

	// Find the maximum count for today
	maxCount := 0
	for _, record := range records {
		recordDate := record[0][:10] // Extract only the date portion (YYYY-MM-DD)
		if recordDate == currentDate && len(record) > 2 {
			count, err := strconv.Atoi(record[2])
			if err == nil && count > maxCount {
				maxCount = count
			}
		}
	}

	return maxCount
}

// checkRecentDuplicate checks if the barcode has been recorded within the last 2 hours
func checkRecentDuplicate(file *os.File, barcodeID string) bool {
	// Go back to the beginning of the file to read all records
	if _, err := file.Seek(0, 0); err != nil {
		fmt.Println("Error seeking to beginning of file:", err)
		return false
	}

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		fmt.Println("Error reading CSV:", err)
		return false
	}

	// Get the current time and the cutoff time for duplicates (2 hours ago)
	now := time.Now()
	twoHoursAgo := now.Add(-2 * time.Hour)

	// Check each record to see if there is a recent duplicate
	for _, record := range records {
		recordTime, err := time.Parse("2006-01-02T15:04:05-07:00", record[0])
		if err != nil {
			fmt.Println("Error parsing timestamp:", err)
			continue
		}

		if record[1] == barcodeID && recordTime.After(twoHoursAgo) {
			return true
		}
	}

	return false
}

// runExportMode handles reading and exporting records from a date or date range
func runExportMode(startDate, endDate string) {
	file, err := os.Open("scanned_barcodes.csv")
	if err != nil {
		fmt.Println("Error opening file:", err)
		return
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		fmt.Println("Error reading CSV:", err)
		return
	}

	// Parse the start date in local time
	location := time.Now().Location()
	start, err := time.ParseInLocation("2006-01-02", startDate, location)
	if err != nil {
		fmt.Println("Error parsing start date:", err)
		return
	}

	// Set the end date for a single day or a range
	var end time.Time
	if endDate == "" {
		end = start.Add(24 * time.Hour).Add(-time.Second) // end of the start day
	} else {
		end, err = time.ParseInLocation("2006-01-02", endDate, location)
		if err != nil {
			fmt.Println("Error parsing end date:", err)
			return
		}
		end = end.Add(24 * time.Hour).Add(-time.Second) // end of the end day
	}

	// Filter records by date range in local time
	var filteredRecords [][]string
	for _, record := range records {
		recordTime, err := time.ParseInLocation("2006-01-02T15:04:05-07:00", record[0], location)
		if err != nil {
			fmt.Println("Error parsing timestamp:", err)
			continue
		}

		if (recordTime.Equal(start) || recordTime.After(start)) && recordTime.Before(end) {
			filteredRecords = append(filteredRecords, record)
		}
	}

	// Handle case where no records are found
	if len(filteredRecords) == 0 {
		fmt.Println("No records found for the specified date range.")
		return
	}

	// Create a dynamic filename with the date range and record count
	var filename string
	if endDate == "" {
		filename = fmt.Sprintf("export_%s_%d_records.csv", startDate, len(filteredRecords))
	} else {
		filename = fmt.Sprintf("export_%s_to_%s_%d_records.csv", startDate, endDate, len(filteredRecords))
	}

	// Export filtered records to the dynamically named file
	exportFile, err := os.Create(filename)
	if err != nil {
		fmt.Println("Error creating export file:", err)
		return
	}
	defer exportFile.Close()

	writer := csv.NewWriter(exportFile)
	if err := writer.WriteAll(filteredRecords); err != nil {
		fmt.Println("Error writing to export file:", err)
	} else {
		fmt.Printf("Exported %d records to %s\n", len(filteredRecords), filename)
	}
	writer.Flush()
}
