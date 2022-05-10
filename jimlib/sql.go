package jimlib

import (
	"database/sql"
	"log"
)

// Event prepared statements
var InsertEvent *sql.Stmt
var GetGoing *sql.Stmt
var GetFlaking *sql.Stmt
var UpdateEventGoing *sql.Stmt
var UpdateEventFlaking *sql.Stmt

// makePreparedStatements adds prepared statements to the database.
//
// The prepared statements are:
//
// InsertEvent - Add an event to the Event table
//
// GetGoing - Get the users going to an event based on the MessageID
//
// GetFlaking - Get the users not going to an event based on the MessageID
//
// UpdateEventGoing - Set the users going to an event based on the MessageID
//
// UpdateEventFlaking - Set the users not going to an event based on the MessageID
func AddPreparedStatements(db *sql.DB) {
	var err error
	InsertEvent, err = db.Prepare(`INSERT INTO Events(MessageID, Title, Date, Details, Going, Flaking)
										VALUES(?,?,?,?,?,?)`)
	if err != nil {
		log.Fatal("failed to create insertEvent prepared statement: ", err)
	}

	GetGoing, err = db.Prepare(`SELECT Going FROM Events WHERE MessageID = ?`)
	if err != nil {
		log.Fatal("failed to create updateEventGoing prepared statement: ", err)
	}

	GetFlaking, err = db.Prepare(`SELECT Flaking FROM Events WHERE MessageID = ?`)
	if err != nil {
		log.Fatal("failed to create updateEventGoing prepared statement: ", err)
	}

	UpdateEventGoing, err = db.Prepare(`UPDATE Events SET Going = ? WHERE MessageID = ?`)
	if err != nil {
		log.Fatal("failed to create updateEventGoing prepared statement: ", err)
	}

	UpdateEventFlaking, err = db.Prepare(`UPDATE Events SET Flaking = ? WHERE MessageID = ?`)
	if err != nil {
		log.Fatal("failed to create updateEventFlaking prepared statement: ", err)
	}
}
