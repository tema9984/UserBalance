package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/tema9984/AvitoInter/internal/model"
)

type EventRepository struct {
	store *Store
}

func (rep *EventRepository) Transaction(user1 model.UserBalance, user2 model.UserBalance, amount int) error {
	rx, _ := rep.store.db.Begin()
	res1, err1 := rx.Exec("UPDATE usersbalance SET balance = $2 WHERE userid = $1", user1.UserID, user1.Balance-amount)
	errHist1 := rep.history(rx, user1.UserID, "debit", "transaction with user "+user2.UserID, amount)
	res2, err2 := rx.Exec("UPDATE usersbalance SET balance = $2 WHERE userid = $1", user2.UserID, user2.Balance+amount)
	errHist2 := rep.history(rx, user2.UserID, "replenishment", "transaction with user "+user1.UserID, amount)
	row1, _ := res1.RowsAffected()
	row2, _ := res2.RowsAffected()
	if err1 == nil && err2 == nil && row1 == 1 && row2 == 1 && errHist1 == nil && errHist2 == nil {
		rx.Commit()
	} else {
		rx.Rollback()
		return fmt.Errorf("transaction not completed")
	}
	return nil
}
func (rep *EventRepository) history(rx *sql.Tx, userid string, operation string, comment string, amount int) error {
	today := time.Now()
	_, err := rx.Exec(`insert into transactionhistory(userid, operation, "comment", amount, "datetime") values ($1, $2, $3, $4, $5)`, userid, operation, comment, amount, today)
	if err != nil {
		logrus.Error(err)
	}
	return err
}
func (rep *EventRepository) CreateUser(user model.UserBalance) error {
	rx, _ := rep.store.db.Begin()
	_, err := rx.Exec("insert into usersbalance (UserID, Balance) values ($1, $2)", user.UserID, user.Balance)
	errHist := rep.history(rx, user.UserID, "replenishment", "bank transfer", user.Balance)
	if err != nil || errHist != nil {
		rx.Rollback()
		return fmt.Errorf("error while adding to database")
	} else {
		rx.Commit()
	}
	return nil
}
func (rep *EventRepository) Replenishment(user model.UserBalance, amount int) error {
	operation := "replenishment"
	comment := "bank transfer"
	if amount < 0 {
		operation = "debit"
		comment = "buy of service"
	}
	rx, _ := rep.store.db.Begin()
	_, err := rx.Exec("UPDATE usersbalance SET balance = $2 WHERE userid = $1", user.UserID, user.Balance)
	errHist := rep.history(rx, user.UserID, operation, comment, amount)
	if err != nil || errHist != nil {
		rx.Rollback()
		return fmt.Errorf("error while adding to database")
	} else {
		rx.Commit()
	}
	return nil
}

func (rep *EventRepository) AllUser() (userBalance map[string]model.UserBalance) {
	userBalance = map[string]model.UserBalance{}
	rows, err := rep.store.db.Query("select * from usersbalance")
	if err != nil {
		logrus.Error(err)
		return
	}
	for rows.Next() {
		usBal := model.UserBalance{}
		err := rows.Scan(&usBal.UserID, &usBal.Balance)
		if err != nil {
			logrus.Warn(err)
			continue
		}
		userBalance[usBal.UserID] = usBal
	}
	return
}
func (rep *EventRepository) HistoryByID(userID string) []model.History {
	userHistory := []model.History{}
	rows, err := rep.store.db.Query(`select userid, operation, "comment", amount, datetime from transactionhistory where userid='` + userID + "'")
	if err != nil {
		logrus.Error(err)
		return nil
	}
	for rows.Next() {
		operation := model.History{}
		err := rows.Scan(&operation.UserID, &operation.Operation, &operation.Comment, &operation.Amount, &operation.Date)
		if err != nil {
			logrus.Warn(err)
			continue
		}
		userHistory = append(userHistory, operation)
	}
	return userHistory
}
