package model

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
)

// Key store class
type UserKey struct {
	userMention string
	keyText     string
	keyType     string
	userID      int
}

func SetKey(db *sql.DB, senderID int, mentionID string, payloadMsg string) (string, error) {

	var keyText string
	var keyType string

	payloadMsg = strings.Replace(payloadMsg, "/set_key ", "", -1)
	//Get the type of key
	var strParts = strings.Split(payloadMsg, " ")
	for _, pair := range strParts {
		tokens := strings.Split(pair, "=")
		if strings.ToLower(tokens[0]) == "type" {
			keyType = tokens[1]
		}
	}
	//Get the start of key
	if strings.Contains(payloadMsg, "-----BEGIN") {
		keyText = payloadMsg[strings.Index(payloadMsg, "-----BEGIN"):]
	}
	if strings.Contains(payloadMsg, "----- BEGIN") {
		keyText = payloadMsg[strings.Index(payloadMsg, "----- BEGIN"):]
	}
	if strings.Contains(payloadMsg, "ssh-rsa") {
		keyText = payloadMsg[strings.Index(payloadMsg, "ssh-rsa"):]
	}

	uk := UserKey{
		keyText: keyText,
		keyType: keyType,
		userID:  senderID,
	}

	if db == nil {
		log.Printf("db is nil")
	}

	var err = db.Ping()
	if err == nil {
		log.Printf("No ping to db")
	} else {
		log.Printf("Ping successful")
	}
	stmt, dberr := db.Prepare("INSERT INTO keys(userid, keytype, keytext) VALUES($1,$2,$3)")
	if dberr != nil {
		log.Fatal(dberr)
	}
	res, dberr := stmt.Exec(uk.userID, uk.keyType, uk.keyText)
	if dberr != nil {
		return "Error saving", dberr
		log.Fatal(dberr)
	}
	rowCnt, dberr := res.RowsAffected()
	if err != nil {
		log.Fatal(dberr)
	}
	return fmt.Sprintf("Added = %d key\n", rowCnt), nil

}
