package main

import (
	"database/sql"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"gotest.tools/v3/assert"
)

const confName = "thisIsATest@test.example.com"

var sqlDB *sql.DB

func TestMain(m *testing.M) {
	var err error
	if err := initDatabase(); err != nil {
		log.Fatal("cannot initialize database")
	}
	sqlDB, err = sql.Open("sqlite3", *sqlitePath)
	if err != nil {
		log.Fatal("cannot open database")
	}

	exitCode := m.Run()

	sqlDB.Close()
	sqlDB = nil
	os.Remove("conferencemapper.db")
	os.Exit(exitCode)
}

func TestGetRandomConfID(t *testing.T) {
	for i := 0; i < 300; i++ {
		result := getConfId(sqlDB, fmt.Sprintf("TestGetRandomConfID%d", i))
		assert.Check(t, result > int(math.Pow10(*xDigitIDs-1)))
		assert.Check(t, result < int(math.Pow10(*xDigitIDs)))
	}
}

func TestMapperJustConfName(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "/conferenceMapper?conference="+confName, nil)
	assert.NilError(t, err)

	confNameSanizied := sanitizeConferenceName(confName)
	// in this test confName has uppercase letters which will be changed to lowercase only
	assert.Check(t, confName != confNameSanizied)

	inserted := insertConference(sqlDB, confNameSanizied, 12553)
	assert.Check(t, inserted)

	rec := httptest.NewRecorder()
	handler := http.HandlerFunc(mapper)
	handler.ServeHTTP(rec, req)

	assert.Assert(t, rec.Code == http.StatusOK)
	assert.Equal(t, rec.Body.String(), `{"id":12553,"conference":"`+confNameSanizied+`"}`+"\n")
}

func TestMapperUnknownConfID(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "/conferenceMapper?id=01234", nil)
	assert.NilError(t, err)

	rec := httptest.NewRecorder()
	handler := http.HandlerFunc(mapper)
	handler.ServeHTTP(rec, req)

	assert.Assert(t, rec.Code == http.StatusOK)
	assert.Equal(t, rec.Body.String(), `{"id":1234,"conference":"false"}`+"\n")
}

func TestMapperConfIDAndConfName(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "/conferenceMapper?id=1234&conference=test@example.com", nil)
	assert.NilError(t, err)

	rec := httptest.NewRecorder()
	handler := http.HandlerFunc(mapper)
	handler.ServeHTTP(rec, req)

	assert.Assert(t, rec.Code == http.StatusOK)
	assert.Equal(t, rec.Body.String(), `{"id":1234,"conference":"false"}`+"\n")
}
