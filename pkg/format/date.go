package format

import "time"

func DateRange(args []string) (startDate, endDate string) {
	now := time.Now()
	for i, arg := range args {
		if (arg == "--start" || arg == "-s") && i+1 < len(args) {
			startDate = args[i+1]
		} else if (arg == "--end" || arg == "-e") && i+1 < len(args) {
			endDate = args[i+1]
		} else if (arg == "--month" || arg == "-m") && i+1 < len(args) {
			monthStr := args[i+1]
			if monthTime, err := time.Parse("2006-01", monthStr); err == nil {
				startDate = monthTime.Format("2006-01-02")
				endDate = monthTime.AddDate(0, 1, -1).Format("2006-01-02")
			}
		}
	}

	if startDate == "" && endDate == "" {
		startDate = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
		endDate = time.Date(now.Year(), now.Month()+1, 0, 23, 59, 59, 0, now.Location()).Format("2006-01-02")
	} else if startDate != "" && endDate == "" {
		endDate = now.Format("2006-01-02")
	} else if startDate == "" && endDate != "" {
		startDate = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
	}

	return startDate, endDate
}

func DateForDisplay(dateStr string) string {
	if dateStr == "" {
		return ""
	}
	if date, err := time.Parse("2006-01-02", dateStr); err == nil {
		return date.Format("January 2, 2006")
	}
	return dateStr
}
