package db

import (
	"database/sql"
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// NewDBSQLHandler creates a new database handler for PostgreSQL using GORM.
func NewDBSQLHandler(conn string) (*gorm.DB, error) {
	sqlDB, err := sql.Open("postgres", conn)
	if err != nil {
		return nil, err
	}

	// Ping the database to check if the connection is successful
	err = sqlDB.Ping()
	if err != nil {
		return nil, err
	}
	fmt.Println("Connected to the PostgreSQL database!")
	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn: sqlDB,
	}), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	return gormDB, nil
}

// DBRollback handles the rollback and commit of a database transaction.
// It recovers from a panic, rolls back the transaction if an error occurs,
// and commits the transaction if no errors are present.
func DBRollback(tx *gorm.DB, err *error) {
	if p := recover(); p != nil {
		if rbErr := tx.Rollback().Error; rbErr != nil {
			fmt.Printf("DBRollback: %s", rbErr.Error())
		}
		*err = fmt.Errorf("transaction: %v", p)
	} else if *err != nil {
		if rbErr := tx.Rollback().Error; rbErr != nil {
			fmt.Printf("DBRollback: %s", rbErr.Error())
		}
	} else {
		*err = tx.Commit().Error
	}
}
