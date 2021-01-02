package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html"
	"math/rand"
	"net/http"
	"strconv"

	log "github.com/sirupsen/logrus"

	_ "github.com/mattn/go-sqlite3"
)

var sqlitePath = "conferencemapper.db"

//var sqlDb *sql.DB

type ConferenceMapperResult struct {
	ConferenceID   int
	ConferenceName string
}

func mapper(w http.ResponseWriter, r *http.Request) {
	result := ConferenceMapperResult{}
	defer json.NewEncoder(w).Encode(&result)

	conference := r.URL.Query().Get("conference")
	paramID := r.URL.Query().Get("id")

	log.WithFields(log.Fields{
		"conference": conference,
		"paramID":    paramID,
	}).Info("mapper(conference, id)")

	sqlDb, err := sql.Open("sqlite3", sqlitePath)
	if err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Error("mapper: Connect to database")
		return
	}
	defer sqlDb.Close()

	if paramID != "" {
		confId, err := strconv.ParseInt(paramID, 10, 0)
		result.ConferenceID = int(confId)

		if err != nil {
			log.WithFields(log.Fields{
				"paramID": paramID,
				"confID":  result.ConferenceID,
			}).Error("Parsing of confID failed")
		}
		// conference given
		result.ConferenceName = getConfName(sqlDb, result.ConferenceID)
		log.WithFields(log.Fields{
			"confID":   result.ConferenceID,
			"confName": result.ConferenceName,
		}).Debug("Parsed Conf name")
	}
	if conference != "" {
		// conference given
		result.ConferenceName = conference
		result.ConferenceID = getConfId(sqlDb, conference)
	}

	updateConferenceUsage(sqlDb, result.ConferenceID)

	log.WithFields(log.Fields{
		"result": result,
	}).Info("mapper return")
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
				result = rand.Intn(999999-100000) + 100000
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
		return ""
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

func main() {
	sqlDb, err := sql.Open("sqlite3", sqlitePath)
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
		fmt.Fprintf(w, "Conference Mapper (for jitsi) is running %s", html.EscapeString(r.URL.Path))
	})

	http.HandleFunc("/conferenceMapper", mapper)

	log.Info("Listen on 8001")
	log.Fatal(http.ListenAndServe(":8001", nil))
}
