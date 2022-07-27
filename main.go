package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	_ "github.com/mattn/go-sqlite3"
)

var (
	sqlitePath = flag.String("dbPath", "conferencemapper.db", "Path to SQLite Database")
	xDigitIDs  = flag.Int("xDigitIDs", 7, "Number of digits for new random conference IDs")
)

//var sqlDb *sql.DB

type ConferenceMapperResult struct {
	ConferenceID   int    `json:"id"`
	ConferenceName string `json:"conference"`
}

func mapper(w http.ResponseWriter, r *http.Request) {
	result := ConferenceMapperResult{}
	defer sendResponse(w, result)

	conference := r.URL.Query().Get("conference")
	paramID := r.URL.Query().Get("id")

	log.WithFields(log.Fields{
		"conference": conference,
		"paramID":    paramID,
	}).Info("mapper(conference, id)")

	sqlDb, err := sql.Open("sqlite3", *sqlitePath)
	if err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Error("mapper: Connect to database")
		return
	}
	defer sqlDb.Close()

	if paramID != "" {
		confId, err := strconv.ParseInt(paramID, 10, 0)
		if err != nil {
			log.WithFields(log.Fields{
				"paramID": paramID,
				"confID":  confId,
			}).Error("Parsing of confID failed")
		}

		result.ConferenceID = int(confId)
		result.ConferenceName = strings.ToLower(getConfName(sqlDb, result.ConferenceID))
		log.WithFields(log.Fields{
			"confID":   result.ConferenceID,
			"confName": result.ConferenceName,
		}).Debug("Parsed Conf name")
	}

	// only set new conference name if not set via conf id
	if conference != "" && result.ConferenceName == "" {
		// sanitize <roomname>@conference.example.com
		parts := strings.Split(conference, "@")
		room := strings.ToLower(strings.Join(parts[0:len(parts)-1], "@"))
		result.ConferenceName = url.QueryEscape(room) + "@" + parts[len(parts)-1]
		result.ConferenceID = getConfId(sqlDb, result.ConferenceName)
	}

	updateConferenceUsage(sqlDb, result.ConferenceID)
}

func sendResponse(w http.ResponseWriter, result ConferenceMapperResult) {
	if err := json.NewEncoder(w).Encode(&result); err != nil {
		log.WithFields(log.Fields{
			"result": result,
			"error":  err,
		}).Error("Encoding of response failed")
	}

	log.WithFields(log.Fields{
		"result": result,
	}).Info("sendResponse()")
}

func getConfId(db *sql.DB, confName string) int {
	log.WithFields(log.Fields{
		"confName": confName,
	}).Debug("getConfId(confName)")
	var result int
	row := db.QueryRow("SELECT conferenceId FROM conferences WHERE conferenceName = ?", confName)
	if err := row.Scan(&result); err != nil {
		if err == sql.ErrNoRows {
			// generate new ID and return that
			for {
				result = rand.Intn(int(math.Pow10(*xDigitIDs))-1-int(math.Pow10(*xDigitIDs-1))) + int(math.Pow10(*xDigitIDs-1))
				log.WithFields(log.Fields{
					"confName": confName,
					"confId":   result,
				}).Debug("getConfId(confName) store random confID")
				if insertConference(db, confName, result) {
					// insertion worked; return it
					return result
				}
			}
		}
		log.WithFields(log.Fields{
			"confName": confName,
			"err":      err,
		}).Error("Could not get data conf id from db")
		return -1
	}
	return result
}

func getConfName(db *sql.DB, confId int) string {
	log.WithFields(log.Fields{
		"confId": confId,
	}).Debug("getConfName(confId)")
	var result string
	row := db.QueryRow("SELECT conferenceName FROM conferences WHERE conferenceId = ?", confId)
	if err := row.Scan(&result); err != nil {
		log.WithFields(log.Fields{
			"confId": confId,
			"err":    err,
		}).Error("Could not query conf name from db")
		return "false"
	}
	return result
}

// returns true if insertion completed
func insertConference(db *sql.DB, confName string, confId int) bool {
	log.WithFields(log.Fields{
		"confName": confName,
		"confId":   confId,
	}).Debug("insertConference(confName,confId)")
	stmt, err := db.Prepare("INSERT INTO conferences(conferenceName, conferenceId) VALUES (?, ?)")
	if err != nil {
		log.WithFields(log.Fields{
			"confName": confName,
			"confId":   confId,
			"err":      err,
		}).Error("Could not insert conf to db")
		return false
	}
	_, err = stmt.Exec(confName, confId)
	if err != nil {
		log.WithFields(log.Fields{
			"confName": confName,
			"confId":   confId,
			"err":      err,
		}).Error("Could not insert conf to db (exec stmt)")
		return false
	}
	return true
}

func updateConferenceUsage(db *sql.DB, confId int) bool {
	_, err := db.Exec("UPDATE conferences set lastUsed = (strftime('%s','now')) WHERE conferenceId = ?", confId)
	log.WithFields(log.Fields{
		"confId": confId,
		"err":    err,
	}).Debug("updateConferenceUsage()")
	if err != nil {
		return false
	}
	return true
}

func cleanupOldEntries() {
	for {
		time.Sleep(24 * time.Hour)
		log.Info("Run cleanup of old entries")

		sqlDb, err := sql.Open("sqlite3", *sqlitePath)
		if err != nil {
			log.WithFields(log.Fields{
				"err": err,
			}).Error("cleanupOldEntries: Connect to database")
			return
		}

		oldTime := time.Now().Add(24 * time.Hour * 365).Unix()
		_, err = sqlDb.Exec("DELETE FROM conferences WHERE lastUsed < ?", oldTime)
		if err != nil {
			log.WithFields(log.Fields{
				"err": err,
			}).Error("cleanupOldEntries: Run Cleanup")
			return
		}
		sqlDb.Close()
	}
}

func main() {
	flag.Parse()
	rand.Seed(time.Now().Unix())

	sqlDb, err := sql.Open("sqlite3", *sqlitePath)
	if err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Fatal("main: Open sql db")
	}

	stmt, err := sqlDb.Prepare(`CREATE TABLE IF NOT EXISTS conferences (
		"conferenceId" INTEGER(6) NOT NULL PRIMARY KEY,
		"conferenceName" TEXT NOT NULL UNIQUE,
		"created" INTEGER(4) NOT NULL DEFAULT (strftime('%s','now')),
		"lastUsed" INTEGER(4) NOT NULL DEFAULT (strftime('%s','now'))
	)`)
	if err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Fatal("main: Create db statement")
	}
	stmt.Exec()
	sqlDb.Close()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Conference Mapper (for jitsi) is running")
	})

	http.HandleFunc("/conferenceMapper", mapper)

	log.Info("Listen on 8001")
	log.Fatal(http.ListenAndServe(":8001", nil))
}
