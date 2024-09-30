package utils

import (
	"database/sql"
	"fmt"
	"github.com/donnyhardyanto/dxlib/log"
	"github.com/jmoiron/sqlx"
	"strings"
)

func FormatIdentifier(identifier string, driverName string) string {
	// Convert the identifier to lowercase as the base case
	formattedIdentifier := strings.ToLower(identifier)

	// Apply database-specific formatting
	switch driverName {
	case "oracle", "db2":
		formattedIdentifier = strings.ToUpper(formattedIdentifier)
		return formattedIdentifier
	}

	// Wrap the identifier in quotes to preserve case in the SQL statement
	return `"` + formattedIdentifier + `"`
}

func DeformatIdentifier(identifier string, driverName string) string {
	// Remove the quotes from the identifier
	deformattedIdentifier := strings.Trim(identifier, `"`)
	deformattedIdentifier = strings.ToLower(deformattedIdentifier)
	return deformattedIdentifier
}

func DeformatKeys(kv map[string]interface{}, driverName string) (r map[string]interface{}) {
	r = map[string]interface{}{}
	for k, v := range kv {
		r[DeformatIdentifier(k, driverName)] = v
	}
	return r
}

func PrepareArrayArgs(keyValues map[string]any, driverName string) (fieldNames string, fieldValues string, fieldArgs []any) {
	for k, v := range keyValues {
		if fieldNames != "" {
			fieldNames += ", "
			fieldValues += ", "
		}

		fieldName := FormatIdentifier(k, driverName)
		fieldNames += fieldName
		fieldValues += ":" + fieldName

		var s sql.NamedArg
		switch v.(type) {
		case bool:
			switch driverName {
			case "oracle", "sqlserver":
				if v.(bool) == true {
					keyValues[k] = 1
				} else {
					keyValues[k] = 0
				}

			default:
			}

		default:
		}
		s = sql.Named(fieldName, keyValues[k])
		fieldArgs = append(fieldArgs, s)
	}

	return fieldNames, fieldValues, fieldArgs
}

// Function to kill all connections to a specific database

func KillConnections(db *sqlx.DB, dbName string) error {
	query := fmt.Sprintf(`
        SELECT pg_terminate_backend(pg_stat_activity.pid)
        FROM pg_stat_activity
        WHERE pg_stat_activity.datname = '%s'
          AND pid <> pg_backend_pid();
    `, dbName)
	_, err := db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to kill connections: %w", err)
	}
	return nil
}

func DropDatabase(db *sqlx.DB, dbName string) (err error) {
	defer func() {
		if err != nil {
			log.Log.Warnf(`Error drop database %s:%s`, dbName, err.Error())
		}
	}()

	// Kill all connections to the target database
	err = KillConnections(db, dbName)
	if err != nil {
		log.Log.Errorf("Failed to kill connections: %s", err.Error())
		return err
	}

	query := fmt.Sprintf(`DROP DATABASE "%s"`, dbName)
	_, err = db.Exec(query)
	if err != nil {
		return err
	}

	return nil
}

func CreateDatabase(db *sqlx.DB, dbName string) error {
	query := fmt.Sprintf(`CREATE DATABASE "%s"`, dbName)
	_, err := db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}
	return nil
}
