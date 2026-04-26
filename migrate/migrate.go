// migrate imports data from the MySQL dump into a fresh SQLite database.
// Usage: go run ./migrate/migrate.go -dump randomtube_dump.sql -db randomtube.db
package main

import (
	"bufio"
	"database/sql"
	"flag"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"

	dbpkg "randomtube/internal/db"
)

func main() {
	dumpFile := flag.String("dump", "randomtube_dump.sql", "MySQL dump file")
	dbPath := flag.String("db", "randomtube.db", "SQLite output file")
	flag.Parse()

	database, err := dbpkg.Open(*dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer database.Close()

	content, err := os.ReadFile(*dumpFile)
	if err != nil {
		log.Fatalf("read dump: %v", err)
	}
	dump := string(content)

	cats := parseCategories(dump)
	log.Printf("parsed %d categories", len(cats))
	for _, c := range cats {
		_, err := database.Exec(
			`INSERT OR IGNORE INTO categories (id, name, code) VALUES (?, ?, ?)`,
			c.id, c.name, c.code,
		)
		if err != nil {
			log.Printf("insert category %d: %v", c.id, err)
		}
	}

	videos := parseVideos(dump)
	log.Printf("parsed %d videos", len(videos))
	inserted := importVideos(database, videos)
	log.Printf("inserted %d videos", inserted)

	log.Println("migration complete")
}

type catRow struct {
	id   int64
	name string
	code string
}

type videoRow struct {
	id         int64
	youtubeID  string
	statusID   int64
	name       string
	rating     int64
	views      int64
	categoryID sql.NullInt64
}

// parseCategories extracts rows from:
// INSERT INTO `categories` VALUES (id,'name','code',template_id),...
func parseCategories(dump string) []catRow {
	re := regexp.MustCompile(`INSERT INTO \x60categories\x60 VALUES (.+?);`)
	m := re.FindStringSubmatch(dump)
	if m == nil {
		return nil
	}
	return parseCatValues(m[1])
}

func parseCatValues(s string) []catRow {
	// each tuple: (id,'name','code',NULL or int)
	re := regexp.MustCompile(`\((\d+),'((?:[^'\\]|\\.|'')*?)','((?:[^'\\]|\\.|'')*?)',[^)]+\)`)
	matches := re.FindAllStringSubmatch(s, -1)
	var rows []catRow
	for _, m := range matches {
		id, _ := strconv.ParseInt(m[1], 10, 64)
		rows = append(rows, catRow{id: id, name: unescape(m[2]), code: unescape(m[3])})
	}
	return rows
}

// parseVideos extracts rows from the big INSERT for `video` table.
// Format: (id,'youtube_id',status_id,'name',rating,views,category_id)
func parseVideos(dump string) []videoRow {
	// Find the INSERT block — it can be very long, on one line
	re := regexp.MustCompile(`(?s)INSERT INTO \x60video\x60 VALUES (.+?);[\r\n]`)
	m := re.FindStringSubmatch(dump)
	if m == nil {
		log.Println("no video INSERT found")
		return nil
	}
	return parseVideoValues(m[1])
}

func parseVideoValues(s string) []videoRow {
	var rows []videoRow

	// Parse tuple by tuple. Format: (int,'str',int,'str',int,int,int_or_NULL)
	// We'll use a hand-written scanner because regex on 600KB is slow.
	scanner := bufio.NewScanner(strings.NewReader(s))
	scanner.Buffer(make([]byte, 10*1024*1024), 10*1024*1024)
	scanner.Split(bufio.ScanRunes)

	raw := s
	i := 0
	for i < len(raw) {
		// find '('
		for i < len(raw) && raw[i] != '(' {
			i++
		}
		if i >= len(raw) {
			break
		}
		i++ // skip '('

		// id
		id, n := parseInt(raw[i:])
		i += n
		i++ // ','

		// youtube_id
		ytID, n := parseStr(raw[i:])
		i += n
		i++ // ','

		// status_id
		statusID, n := parseInt(raw[i:])
		i += n
		i++ // ','

		// name
		name, n := parseStr(raw[i:])
		i += n
		i++ // ','

		// rating
		rating, n := parseInt(raw[i:])
		i += n
		i++ // ','

		// views
		views, n := parseInt(raw[i:])
		i += n
		i++ // ','

		// category_id (int or NULL)
		catID, n := parseNullInt(raw[i:])
		i += n

		// skip to ')'
		for i < len(raw) && raw[i] != ')' {
			i++
		}
		i++ // skip ')'

		if ytID == "" {
			continue
		}
		rows = append(rows, videoRow{
			id:        id,
			youtubeID: ytID,
			statusID:  statusID,
			name:      name,
			rating:    rating,
			views:     views,
			categoryID: catID,
		})
	}

	return rows
}

func parseInt(s string) (int64, int) {
	i := 0
	neg := false
	if i < len(s) && s[i] == '-' {
		neg = true
		i++
	}
	start := i
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == start {
		return 0, i
	}
	v, _ := strconv.ParseInt(s[start:i], 10, 64)
	if neg {
		v = -v
	}
	return v, i
}

func parseNullInt(s string) (sql.NullInt64, int) {
	if strings.HasPrefix(s, "NULL") {
		return sql.NullInt64{}, 4
	}
	v, n := parseInt(s)
	return sql.NullInt64{Int64: v, Valid: true}, n
}

// parseStr parses a MySQL single-quoted string.
func parseStr(s string) (string, int) {
	if len(s) == 0 || s[0] != '\'' {
		return "", 0
	}
	var b strings.Builder
	i := 1
	for i < len(s) {
		c := s[i]
		if c == '\'' {
			if i+1 < len(s) && s[i+1] == '\'' {
				b.WriteByte('\'')
				i += 2
				continue
			}
			i++ // closing quote
			break
		}
		if c == '\\' && i+1 < len(s) {
			next := s[i+1]
			switch next {
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case 't':
				b.WriteByte('\t')
			case '\\':
				b.WriteByte('\\')
			case '\'':
				b.WriteByte('\'')
			default:
				b.WriteByte(next)
			}
			i += 2
			continue
		}
		b.WriteByte(c)
		i++
	}
	return b.String(), i
}

func unescape(s string) string {
	s = strings.ReplaceAll(s, "\\'", "'")
	s = strings.ReplaceAll(s, "\\\\", "\\")
	return s
}

func importVideos(database *sql.DB, videos []videoRow) int {
	tx, err := database.Begin()
	if err != nil {
		log.Fatal(err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT OR IGNORE INTO videos
		(id, youtube_id, name, category_id, enabled, views, rating)
		VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()

	count := 0
	for _, v := range videos {
		enabled := 1
		if v.statusID != 1 {
			enabled = 0
		}
		var catID any
		if v.categoryID.Valid {
			catID = v.categoryID.Int64
		}
		res, err := stmt.Exec(v.id, v.youtubeID, v.name, catID, enabled, v.views, v.rating)
		if err != nil {
			log.Printf("insert video %s: %v", v.youtubeID, err)
			continue
		}
		n, _ := res.RowsAffected()
		count += int(n)
	}

	if err := tx.Commit(); err != nil {
		log.Fatalf("commit: %v", err)
	}
	return count
}
