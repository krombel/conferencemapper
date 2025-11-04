package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var (
	sqlitePath = flag.String("dbPath", "conferencemapper.db", "Path to SQLite Database")
	xDigitIDs  = flag.Int("xDigitIDs", 7, "Number of digits for new random conference IDs")
)

// var sqlDb *sql.DB
type ConferenceMapperResult struct {
	ConferenceID   int    `json:"id"` // PIN with potentially leading zeroes
	ConferenceName string `json:"conference"`
}

func mapper(w http.ResponseWriter, r *http.Request) {
	result := ConferenceMapperResult{}
	defer sendResponse(w, &result)

	conference := r.URL.Query().Get("conference")
	// for log only
	conferenceEscaped := url.QueryEscape(conference)

	paramID := r.URL.Query().Get("id")
	if paramID != "" {
		confId, err := strconv.Atoi(paramID)
		if err != nil {
			slog.With(
				"paramID", paramID,
				"confID", confId,
				"err", err,
			).Error("Parsing of confID failed")
			return
		}
		result.ConferenceID = confId
	}

	slog.With(
		"conference", conferenceEscaped,
		"ConferenceID", result.ConferenceID,
	).Info("mapper(conference, id)")

	sqlDb, err := sql.Open("sqlite3", *sqlitePath)
	if err != nil {
		slog.With(
			"err", err,
		).Error("mapper: Connect to database")
		return
	}
	defer func() {
		err := sqlDb.Close()
		if err != nil {
			slog.With(
				"err", err,
			).Error("mapper: Close database")
			return
		}
	}()

	if result.ConferenceID != 0 {
		result.ConferenceName = strings.ToLower(getConfName(sqlDb, result.ConferenceID))
		slog.With(
			"confID", result.ConferenceID,
			"confName", result.ConferenceName,
		).Debug("Parsed Conf name")
	}

	// only set new conference name if not set via conf id
	if conference != "" && result.ConferenceName == "" {
		result.ConferenceName = sanitizeConferenceName(conference)
		result.ConferenceID = getConfId(sqlDb, result.ConferenceName)
	}

	updateConferenceUsage(sqlDb, result.ConferenceID)
}

func sendResponse(w http.ResponseWriter, result *ConferenceMapperResult) {
	if err := json.NewEncoder(w).Encode(&result); err != nil {
		slog.With(
			"result", result,
			"error", err,
		).Error("Encoding of response failed")
	}

	slog.With(
		"result", *result,
	).Info("sendResponse()")
}

func getConfId(db *sql.DB, confName string) int {
	slog.With(
		"confName", confName,
	).Debug("getConfId(confName)")
	var result int
	row := db.QueryRow("SELECT conferenceId FROM conferences WHERE conferenceName = ?", confName)
	if err := row.Scan(&result); err != nil {
		if err == sql.ErrNoRows {
			// generate new ID and return that
			for {
				result = rand.Intn(int(math.Pow10(*xDigitIDs))-1-int(math.Pow10(*xDigitIDs-1))) + int(math.Pow10(*xDigitIDs-1))
				slog.With(
					"confName", confName,
					"confId", result,
				).Debug("getConfId(confName) store random confID")
				if insertConference(db, confName, result) {
					// insertion worked; return it
					return result
				}
			}
		}
		slog.With(
			"confName", confName,
			"err", err,
		).Error("Could not get data conf id from db")
		return -1
	}
	return result
}

// sanitizeConferenceName takes roomName@domain and sanizites it to a format used by jitsi
func sanitizeConferenceName(conference string) string {
	parts := strings.Split(conference, "@")
	room := strings.ToLower(strings.Join(parts[0:len(parts)-1], "@"))
	return url.QueryEscape(room) + "@" + parts[len(parts)-1]
}

func getConfName(db *sql.DB, confId int) string {
	slog.With(
		"confId", confId,
	).Debug("getConfName(confId)")
	var result string
	row := db.QueryRow("SELECT conferenceName FROM conferences WHERE conferenceId = ?", confId)
	if err := row.Scan(&result); err != nil {
		slog.With(
			"confId", confId,
			"err", err,
		).Error("Could not query conf name from db")
		return "false"
	}
	return result
}

// returns true if insertion completed
func insertConference(db *sql.DB, confName string, confId int) bool {
	slog.With(
		"confName", confName,
		"confId", confId,
	).Debug("insertConference(confName,confId)")
	stmt, err := db.Prepare("INSERT INTO conferences(conferenceName, conferenceId) VALUES (?, ?)")
	if err != nil {
		slog.With(
			"confName", confName,
			"confId", confId,
			"err", err,
		).Error("Could not insert conf to db")
		return false
	}
	_, err = stmt.Exec(confName, confId)
	if err != nil {
		slog.With(
			"confName", confName,
			"confId", confId,
			"err", err,
		).Error("Could not insert conf to db (exec stmt)")
		return false
	}
	return true
}

func updateConferenceUsage(db *sql.DB, confId int) bool {
	_, err := db.Exec("UPDATE conferences set lastUsed = (strftime('%s','now')) WHERE conferenceId = ?", confId)
	slog.With(
		"confId", confId,
		"err", err,
	).Debug("updateConferenceUsage()")
	return err == nil
}

func cleanupOldEntries() {
	for {
		time.Sleep(24 * time.Hour)
		slog.Info("Run cleanup of old entries")

		sqlDb, err := sql.Open("sqlite3", *sqlitePath)
		if err != nil {
			slog.With(
				"err", err,
			).Error("cleanupOldEntries: Connect to database")
			continue
		}

		oldTime := time.Now().Add(-24 * time.Hour * 365).Unix()
		_, err = sqlDb.Exec("DELETE FROM conferences WHERE lastUsed < ?", oldTime)
		if err != nil {
			slog.With(
				"err", err,
			).Error("cleanupOldEntries: Run Cleanup")
			continue
		}
		err = sqlDb.Close()
		if err != nil {
			slog.With(
				"err", err,
			).Error("cleanupOldEntries: Close database")
			continue
		}
	}
}

func initDatabase() error {
	sqlDb, err := sql.Open("sqlite3", *sqlitePath)
	if err != nil {
		slog.With(
			"err", err,
		).Error("main: Open sql db")
		return err
	}

	stmt, err := sqlDb.Prepare(`CREATE TABLE IF NOT EXISTS conferences (
		"conferenceId" INTEGER(6) NOT NULL PRIMARY KEY,
		"conferenceName" TEXT NOT NULL UNIQUE,
		"created" INTEGER(4) NOT NULL DEFAULT (strftime('%s','now')),
		"lastUsed" INTEGER(4) NOT NULL DEFAULT (strftime('%s','now'))
	)`)
	if err != nil {
		slog.With(
			"err", err,
		).Error("main: Create db statement (Prepare)")
		return err
	}
	_, err = stmt.Exec()
	if err != nil {
		slog.With(
			"err", err,
		).Error("main: Create db statement (Execute)")
		return err
	}
	err = sqlDb.Close()
	if err != nil {
		slog.With(
			"err", err,
		).Error("main: Close db")
		return err
	}
	return err
}

func main() {
	flag.Parse()

	if err := initDatabase(); err != nil {
		slog.With("error", err).Error("cannot initialize database")
		os.Exit(1)
	}

	go cleanupOldEntries()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, err := fmt.Fprintf(w, "Conference Mapper (for jitsi) is running")
		if err != nil {
			slog.With(
				"err", err,
			).Error("Root handler failed")
		}
	})

	http.HandleFunc("/conferenceMapper", mapper)

	slog.Info("Listen on 8001")
	slog.Error("closed", "err", http.ListenAndServe(":8001", nil))
}
