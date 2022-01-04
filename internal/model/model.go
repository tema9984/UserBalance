package model

import "time"

type Respons struct {
	Result []UserBalance
}

type ErrRespons struct {
	Err error `json:"error"`
}
type UserBalance struct {
	UserID  string `json:"user_id"`
	Balance int    `json:"balance"`
}
type UserBalanceConv struct {
	UserID  string  `json:"user_id"`
	Balance float64 `json:"balance"`
}
type TransUser struct {
	User1ID string `json:"user1_id"`
	User2ID string `json:"user2_id"`
	Amount  int    `json:"amount"`
}
type History struct {
	UserID    string
	Operation string
	Comment   string
	Amount    int
	Date      time.Time
}
type Currency struct {
	Rub Data `json:"data"`
}
type Data struct {
	Rub float64 `json:"RUB"`
}
